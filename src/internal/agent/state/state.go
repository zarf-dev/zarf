package state

import (
	"encoding/json"
	"os"

	"github.com/defenseunicorns/zarf/src/types"
)

const zarfStatePath = "/etc/zarf-state/state"

// GetZarfStateFromAgentPod reads the state json file that was mounted into the agent pods.
func GetZarfStateFromAgentPod() (types.ZarfState, error) {
	zarfState := types.ZarfState{}

	// Read the state file
	stateFile, err := os.ReadFile(zarfStatePath)
	if err != nil {
		return zarfState, err
	}

	// Unmarshal the json file into a Go struct
	return zarfState, json.Unmarshal(stateFile, &zarfState)
}
