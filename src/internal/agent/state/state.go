// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package state provides helpers for interacting with the Zarf agent state.
package state

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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

// GetServiceInfoFromRegistryAddress gets the service info for a registry address if it is a NodePort
func GetServiceInfoFromRegistryAddress(stateRegistryAddress string) (string, error) {
	registryAddress := stateRegistryAddress
	c, err := cluster.NewCluster()
	if err != nil {
		return "", fmt.Errorf("unable to get service information for the registry %q: %w", stateRegistryAddress, err)
	}

	// If this is an internal service then we need to look it up and
	registryServiceInfo, err := c.ServiceInfoFromNodePortURL(stateRegistryAddress)
	if err != nil {
		message.Debugf("registry appears to not be a nodeport service, using original address %q", stateRegistryAddress)
	} else {
		registryAddress = fmt.Sprintf("%s:%d", registryServiceInfo.ClusterIP, registryServiceInfo.Port)
	}

	return registryAddress, nil
}
