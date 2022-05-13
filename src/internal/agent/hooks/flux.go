package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
	v1 "k8s.io/api/admission/v1"
)

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			var patchOperations []operations.PatchOperation

			type GenericGitRepo struct {
				Spec struct {
					URL string `json:"url"`
				}
			}

			// parse to simple struct to read the git url
			gitRepo := &GenericGitRepo{}
			if err := json.Unmarshal(r.Object.Raw, &gitRepo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal manifest: %v", err)
			}

			message.Info(gitRepo.Spec.URL)

			replacedURL := git.MutateGitUrlsInText("http://zarf-gitea-http.zarf.svc.cluster.local:3000", gitRepo.Spec.URL)
			patchOperations = append(patchOperations, operations.ReplacePatchOperation("/spec/url", replacedURL))

			return &operations.Result{
				Allowed:  true,
				PatchOps: patchOperations,
			}, nil
		},
	}
}
