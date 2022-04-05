package pods

import (
	"encoding/json"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/message"
	v1 "k8s.io/api/core/v1"
)

// NewMutationHook creates a new instance of pods mutation hook
func NewMutationHook() operations.Hook {
	message.Debug("pods.NewMutationHook()")
	return operations.Hook{
		Create: mutateCreate(),
	}
}

func parsePod(object []byte) (*v1.Pod, error) {
	message.Debugf("pods.parsePod(%s)", string(object))

	var pod v1.Pod
	if err := json.Unmarshal(object, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}
