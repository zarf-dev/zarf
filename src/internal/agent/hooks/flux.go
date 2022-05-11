package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	v1 "k8s.io/api/admission/v1"

	corev1 "k8s.io/api/core/v1"
)

// NewPodMutationHook creates a new instance of pods mutation hook
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			var patchOperations []operations.PatchOperation

			return &operations.Result{
				Allowed:  true,
				PatchOps: patchOperations,
			}, nil
		},
	}
}
