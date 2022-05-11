package hooks

import (
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	v1 "k8s.io/api/admission/v1"
)

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			var patchOperations []operations.PatchOperation

			message.Debug(r)

			return &operations.Result{
				Allowed:  true,
				PatchOps: patchOperations,
			}, nil
		},
	}
}
