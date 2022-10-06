package hooks

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"

	corev1 "k8s.io/api/core/v1"
)

// NewPodMutationHook creates a new instance of pods mutation hook
func NewPodMutationHook() operations.Hook {
	message.Debug("hooks.NewMutationHook()")
	return operations.Hook{
		Create: mutatePod,
		Update: mutatePod,
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

func mutatePod(r *v1.AdmissionRequest) (*operations.Result, error) {
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

	zarfState, err := getStateFromAgentPod(zarfStatePath)
	if err != nil {
		message.Debugf("Unable to load the ZarfState file so that the Agent can mutate pods: %#v", err)
		return nil, err
	}
	config.InitState(zarfState)
	containerRegistryURL := config.GetRegistry()

	// update the image host for each init container
	for idx, container := range pod.Spec.InitContainers {
		path := fmt.Sprintf("/spec/initContainers/%d/image", idx)
		replacement, err := utils.SwapHost(container.Image, containerRegistryURL)
		if err != nil {
			message.Warnf("Unable to swap the host for (%s)", container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each ephemeral container
	for idx, container := range pod.Spec.EphemeralContainers {
		path := fmt.Sprintf("/spec/ephemeralContainers/%d/image", idx)
		replacement, err := utils.SwapHost(container.Image, containerRegistryURL)
		if err != nil {
			message.Warnf("Unable to swap the host for (%s)", container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each normal container
	for idx, container := range pod.Spec.Containers {
		path := fmt.Sprintf("/spec/containers/%d/image", idx)
		replacement, err := utils.SwapHost(container.Image, containerRegistryURL)
		if err != nil {
			message.Warnf("Unable to swap the host for (%s)", container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
	}

	// Add a label noting the zarf mutation
	patchOperations = append(patchOperations, operations.ReplacePatchOperation("/metadata/labels/zarf-agent", "patched"))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patchOperations,
	}, nil
}

// Reads the state json file that was mounted into the agent pods
func getStateFromAgentPod(zarfStatePath string) (types.ZarfState, error) {
	zarfState := types.ZarfState{}

	// Read the state file
	stateFile, err := os.ReadFile(zarfStatePath)
	if err != nil {
		message.Warnf("Unable to read the zarfState file within the zarf-agent pod.")
		return zarfState, err
	}

	// Unmarshal the json file into a Go struct
	err = json.Unmarshal(stateFile, &zarfState)
	if err != nil {
		message.Warnf("Unable to unmarshal the zarfState file into a useable object.")
		return zarfState, err
	}

	message.Debugf("ZarfState from file = %#v", zarfState)

	return zarfState, err
}
