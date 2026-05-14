// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package operations

import "os"

// MutationMode controls whether the agent mutates resources by default or requires explicit opt-in.
type MutationMode string

const (
	// MutationModeOptOut mutates all resources unless they carry zarf.dev/agent: ignore/skip.
	MutationModeOptOut MutationMode = "opt-out"
	// MutationModeOptIn mutates only resources (or resources in namespaces) labeled zarf.dev/agent: mutate.
	MutationModeOptIn MutationMode = "opt-in"

	agentLabel = "zarf.dev/agent"
)

// ModeFromEnv reads ZARF_AGENT_MUTATION_MODE from the environment, defaulting to opt-in.
func ModeFromEnv() MutationMode {
	if os.Getenv("ZARF_AGENT_MUTATION_MODE") == string(MutationModeOptOut) {
		return MutationModeOptOut
	}
	return MutationModeOptIn
}

// ShouldMutate reports whether the agent should mutate a resource. The resource label takes
// priority over the namespace label; if neither is set, the mode determines the default.
func ShouldMutate(resourceLabels, nsLabels map[string]string, mode MutationMode) bool {
	for _, labels := range []map[string]string{resourceLabels, nsLabels} {
		switch labels[agentLabel] {
		case "mutate":
			return true
		case "ignore", "skip":
			return false
		}
	}
	return mode == MutationModeOptOut
}
