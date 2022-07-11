package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	v1 "k8s.io/api/admission/v1"

	corev1 "k8s.io/api/core/v1"
)

// NewPodMutationHook creates a new instance of pods mutation hook
func NewPodMutationHook() operations.Hook {
	message.Debug("hooks.NewMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			message.Debugf("hooks.mutateCreate()(*v1.AdmissionRequest) - %#v , %s/%s: %#v", r.Kind, r.Namespace, r.Name, r.Operation)

			var patchOperations []operations.PatchOperation
			pod, err := parsePod(r.Object.Raw)
			if err != nil {
				return &operations.Result{Msg: err.Error()}, nil
			}

			if pod.Labels != nil && pod.Labels["zarf-agent"] == "patched" {
				// We've already played with this pod, just keep swimming 🐟
				return &operations.Result{
					Allowed:  true,
					PatchOps: patchOperations,
				}, nil
			}

			// Add the zarf secret to the podspec
			zarfSecret := []corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}}
			patchOperations = append(patchOperations, operations.ReplacePatchOperation("/spec/imagePullSecrets", zarfSecret))

			// update the image host for each init container
			for idx, container := range pod.Spec.InitContainers {
				path := fmt.Sprintf("/spec/initContainers/%d/image", idx)
				replacement := utils.SwapHost(container.Image, "127.0.0.1:31999")
				patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
			}

			// update the image host for each ephemeral container
			for idx, container := range pod.Spec.EphemeralContainers {
				path := fmt.Sprintf("/spec/ephemeralContainers/%d/image", idx)
				replacement := utils.SwapHost(container.Image, "127.0.0.1:31999")
				patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
			}

			// update the image host for each normal container
			for idx, container := range pod.Spec.Containers {
				path := fmt.Sprintf("/spec/containers/%d/image", idx)
				replacement := utils.SwapHost(container.Image, "127.0.0.1:31999")
				patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
			}

			// Add a label noting the zarf mutation
			patchOperations = append(patchOperations, operations.ReplacePatchOperation("/metadata/labels/zarf-agent", "patched"))

			return &operations.Result{
				Allowed:  true,
				PatchOps: patchOperations,
			}, nil
		},
	}
}

func parsePod(object []byte) (*corev1.Pod, error) {
	message.Debugf("pods.parsePod(%s)", string(object))

	var pod corev1.Pod
	if err := json.Unmarshal(object, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}
