// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"slices"
	"strings"
)

// PruneScope defines a resource scope for pruning during deploy or remove.
type PruneScope string

const (
	// PruneScopeClusterResources indicates orphaned cluster resources (charts and components) should be removed.
	PruneScopeClusterResources PruneScope = "cluster-resources"
	// PruneScopeImages indicates orphaned images should be removed from the internal registry.
	PruneScopeImages PruneScope = "images"
	// PruneScopeAll indicates all applicable scopes for the command should be pruned.
	PruneScopeAll PruneScope = "all"
)

// PruneCommand defines the command context for prune scope validation.
type PruneCommand string

const (
	// PruneCommandDeploy indicates deploy command context.
	PruneCommandDeploy PruneCommand = "deploy"
	// PruneCommandRemove indicates remove command context.
	PruneCommandRemove PruneCommand = "remove"
)

// PruneScopes represents a collection of prune scopes.
type PruneScopes []PruneScope

// Contains checks if a given prune scope is present.
func (p PruneScopes) Contains(scope PruneScope) bool {
	return slices.Contains(p, scope)
}

// ClusterResources returns true if cluster-resources pruning is enabled.
func (p PruneScopes) ClusterResources() bool {
	return p.Contains(PruneScopeClusterResources)
}

// Images returns true if images pruning is enabled.
func (p PruneScopes) Images() bool {
	return p.Contains(PruneScopeImages)
}

// Empty returns true if no prune scopes are set.
func (p PruneScopes) Empty() bool {
	return len(p) == 0
}

// Validate checks whether all scopes in p are valid and applicable to cmd.
func (p PruneScopes) Validate(cmd PruneCommand) error {
	for _, scope := range p {
		switch scope {
		case PruneScopeClusterResources:
			if cmd == PruneCommandRemove {
				return fmt.Errorf("prune scope %q is not applicable to package remove", scope)
			}
		case PruneScopeImages:
			// valid for both deploy and remove
		default:
			return fmt.Errorf("unknown prune scope %q", scope)
		}
	}
	return nil
}

// ParsePruneScopes parses and validates a comma-separated prune scope string for the given command.
// An empty string returns empty scopes (pruning disabled).
// "all" or a bare --prune flag expands to all scopes applicable to the command.
func ParsePruneScopes(raw string, cmd PruneCommand) (PruneScopes, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	var result PruneScopes

	addScope := func(s PruneScope) {
		if !result.Contains(s) {
			result = append(result, s)
		}
	}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		scope := PruneScope(part)
		switch scope {
		case PruneScopeAll:
			switch cmd {
			case PruneCommandDeploy:
				addScope(PruneScopeClusterResources)
				addScope(PruneScopeImages)
			case PruneCommandRemove:
				addScope(PruneScopeImages)
			default:
				return nil, fmt.Errorf("unknown prune command %q", cmd)
			}
		case PruneScopeClusterResources:
			if cmd == PruneCommandRemove {
				return nil, fmt.Errorf("prune scope %q is not applicable to package remove", scope)
			}
			addScope(scope)
		case PruneScopeImages:
			addScope(scope)
		default:
			return nil, fmt.Errorf("unknown prune scope %q", scope)
		}
	}

	return result, nil
}
