// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package operations

import (
	"os"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

// ModeFromEnv reads ZARF_AGENT_MUTATION_MODE from the environment
func ModeFromEnv() state.MutationMode {
	if os.Getenv("ZARF_AGENT_MUTATION_MODE") == string(state.MutationModeOptIn) {
		return state.MutationModeOptIn
	}
	return state.MutationModeOptOut
}

// ShouldMutate reports whether the agent should mutate a resource, prioritizing resource labels
func ShouldMutate(resourceLabels, nsLabels map[string]string, mode state.MutationMode) bool {
	for _, labels := range []map[string]string{resourceLabels, nsLabels} {
		switch labels[cluster.AgentLabel] {
		case "mutate":
			return true
		case "ignore", "skip":
			return false
		}
	}
	return mode == state.MutationModeOptOut
}
