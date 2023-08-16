// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state provides helpers for interacting with the Zarf agent state.
package state

import (
	"encoding/json"
	"os"

	"github.com/defenseunicorns/zarf/src/types"
)

const zarfStatePath = "/etc/zarf-state/state"

// GetZarfStateFromAgentPod reads the state json file that was mounted into the agent pods.
func GetZarfStateFromAgentPod() (state *types.ZarfState, err error) {
	// Read the state file
	stateFile, err := os.ReadFile(zarfStatePath)
	if err != nil {
		return nil, err
	}

	// Unmarshal the json file into a Go struct
	return state, json.Unmarshal(stateFile, &state)
}
