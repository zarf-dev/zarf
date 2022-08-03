package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

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

	containerRegistryInfo := config.GetContainerRegistryInfo()

	// TODO @JPERRY: This is where I need to use the config.GetContainerRegistryInfo().RegistryURL
	// update the image host for each init container
	for idx, container := range pod.Spec.InitContainers {
		path := fmt.Sprintf("/spec/initContainers/%d/image", idx)
		replacement := utils.SwapHost(container.Image, containerRegistryInfo.URL)
		patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each ephemeral container
	for idx, container := range pod.Spec.EphemeralContainers {
		path := fmt.Sprintf("/spec/ephemeralContainers/%d/image", idx)
		replacement := utils.SwapHost(container.Image, containerRegistryInfo.URL)
		patchOperations = append(patchOperations, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each normal container
	for idx, container := range pod.Spec.Containers {
		path := fmt.Sprintf("/spec/containers/%d/image", idx)
		replacement := utils.SwapHost(container.Image, containerRegistryInfo.URL)
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
func getZarfStateFromFileWithinAgentPod(zarfStatePath string) (zarfState types.ZarfState, err error) {
	// Read the state file
	stateFile, err := ioutil.ReadFile(zarfStatePath)
	if err != nil {
		message.Warnf("Unable to read the zarfState file within the zarf-agent pod.")
		return zarfState, err
	}

	// Unmarshal the json file into a Go struct
	err = json.Unmarshal([]byte(stateFile), &zarfState)
	if err != nil {
		message.Warnf("Unable to umarshal the zarfState file into a useable object.")
		return zarfState, err
	}

	message.Debugf("ZarfState from file = %#v", zarfState)

	return zarfState, err
}
