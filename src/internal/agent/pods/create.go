package pods

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

func mutateCreate() operations.AdmitFunc {
	message.Debug("pods.mutateCreate()")

	return func(r *v1.AdmissionRequest) (*operations.Result, error) {
		message.Debugf("pods.mutateCreate()(*v1.AdmissionRequest) - %v , %s/%s: %v", r.Kind, r.Namespace, r.Name, r.Operation)

		var patchOperations []operations.PatchOperation
		pod, err := parsePod(r.Object.Raw)
		if err != nil {
			return &operations.Result{Msg: err.Error()}, nil
		}

		if pod.Labels != nil && pod.Labels["zarf"] == "patched" {
			// We've already played with this pod, just keep swimming 🐟
			return &operations.Result{
				Allowed:  true,
				PatchOps: patchOperations,
			}, nil
		}

		// Add the zarf secret to the podspec
		zarfSecret := []corev1.LocalObjectReference{{Name: "zarf-registry"}}
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
		patchOperations = append(patchOperations, operations.ReplacePatchOperation("/metadata/labels/zarf", "patched"))

		return &operations.Result{
			Allowed:  true,
			PatchOps: patchOperations,
		}, nil
	}
}
