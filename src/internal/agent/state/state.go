package state

import (
	"encoding/json"
	"os"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
)

const zarfStatePath = "/etc/zarf-state/state"

// GetZarfStateFromAgentPod reads the state json file that was mounted into the agent pods.
func GetZarfStateFromAgentPod() (types.ZarfState, error) {
	var zarfState types.ZarfState

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
