// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package operations

import (
	"os"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

// PolicyFromEnv reads ZARF_AGENT_MUTATION_POLICY from the environment
func PolicyFromEnv() state.MutationPolicy {
	if os.Getenv("ZARF_AGENT_MUTATION_POLICY") == string(state.MutationPolicyLabeled) {
		return state.MutationPolicyLabeled
	}
	return state.MutationPolicyAll
}

// ShouldMutate reports whether the agent should mutate a resource, prioritizing resource labels
func ShouldMutate(resourceLabels, nsLabels map[string]string, mode state.MutationPolicy) bool {
	for _, labels := range []map[string]string{resourceLabels, nsLabels} {
		switch labels[cluster.AgentLabel] {
		case "mutate":
			return true
		case "ignore", "skip":
			return false
		}
	}
	return mode == state.MutationPolicyAll
}
