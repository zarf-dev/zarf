package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
	v1 "k8s.io/api/admission/v1"
)

const zarfStatePath = "/etc/zarf-state/state"

type SecretRef struct {
	Name string `json:"name"`
}

type GenericGitRepo struct {
	Spec struct {
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
	}
}

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateGitRepoCreate,
		Update: mutateGitRepoUpdate,
	}
}

func mutateGitRepoCreate(r *v1.AdmissionRequest) (*operations.Result, error) {
	var patches []operations.PatchOperation

	zarfState, err := getStateFromAgentPod(zarfStatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load zarf state from file: %w", err)
	}

	// Form the gitServerURL from the state
	gitServerURL := zarfState.GitServer.Address
	message.Debugf("Using the gitServerURL of (%s) to mutate the flux repository", gitServerURL)

	// parse to simple struct to read the git url
	gitRepo := &GenericGitRepo{}
	if err := json.Unmarshal(r.Object.Raw, &gitRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	replacedURL := git.MutateGitUrlsInText(gitServerURL, gitRepo.Spec.URL, zarfState.GitServer.PushUsername)
	message.Debugf("original git URL of (%s) got mutated to (%s)", gitRepo.Spec.URL, replacedURL)

	// Patch updates of the repo spec
	patches = populatePatchOperations(replacedURL, gitRepo.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func mutateGitRepoUpdate(r *v1.AdmissionRequest) (*operations.Result, error) {
	var patches []operations.PatchOperation

	// parse to simple struct to read the git url
	gitRepo := &GenericGitRepo{}
	if err := json.Unmarshal(r.Object.Raw, &gitRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	// Patch updates of the repo spec
	patches = populatePatchOperations(gitRepo.Spec.URL, gitRepo.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populatePatchOperations(repoURL string, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	// If a prior secret exists, replace it
	if secretName != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfGitServerSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfGitServerSecretName}))
	}

	return patches
}
