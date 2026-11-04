// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
)

// ForDeploy creates a noninteractive deployment filter using explicit selectors
// and package defaults. An explicit group selection replaces other group members.
func ForDeploy(optionalComponents string) ComponentFilterStrategy {
	return &deploymentFilter{requestedComponents: helpers.StringToSlice(optionalComponents)}
}

// deploymentFilter is the default filter for deployments.
type deploymentFilter struct {
	requestedComponents []string
}

// Errors for the deployment filter.
var (
	ErrMultipleSameGroup    = fmt.Errorf("cannot specify multiple components from the same group")
	ErrNoDefaultOrSelection = fmt.Errorf("no default or selected component found")
	ErrNotFound             = fmt.Errorf("no compatible components found")
)

// Apply applies the filter.
func (f *deploymentFilter) Apply(pkg api.Package) ([]api.Component, error) {
	var selectedComponents []api.Component
	groupedComponents := map[string][]api.Component{}
	orderedComponentGroups := []string{}

	// Group the components by Name and Group while maintaining order
	for _, component := range pkg.Components {
		groupKey := component.Name
		if component.Group != "" {
			groupKey = component.Group
		}

		if !slices.Contains(orderedComponentGroups, groupKey) {
			orderedComponentGroups = append(orderedComponentGroups, groupKey)
		}

		groupedComponents[groupKey] = append(groupedComponents[groupKey], component)
	}

	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""

	if isPartial {
		matchedRequests := map[string]bool{}

		// NOTE: This does not use forIncludedComponents as it takes group, default and required status into account.
		for _, groupKey := range orderedComponentGroups {
			var groupDefault *api.Component
			var groupSelected *api.Component

			// An explicit choice selects one group member, even when other members
			// are required. Ungrouped required components remain mandatory.
			hasGroupChoice := false
			if len(groupedComponents[groupKey]) > 1 {
				for _, component := range groupedComponents[groupKey] {
					state, err := includedOrExcluded(component.Name, f.requestedComponents, nil)
					if err != nil {
						return nil, err
					}
					if state == included {
						hasGroupChoice = true
						break
					}
				}
			}

			for _, component := range groupedComponents[groupKey] {
				// Ensure we have a local version of the component to point to (otherwise the pointer might change on us)
				component := component

				selectState, err := includedOrExcluded(component.Name, f.requestedComponents, matchedRequests)
				if err != nil {
					return nil, err
				}

				if component.Optional || hasGroupChoice {
					if selectState == excluded {
						// Explicit exclusions take precedence over inclusions.
						continue
					} else if selectState == unknown && component.Default && groupDefault == nil {
						// If the component is default but not included or excluded, remember the default
						groupDefault = &component
					}
				} else {
					// Force the selectState to included for Required components
					selectState = included
				}

				if selectState == included {
					// Then check for already selected groups
					if groupSelected != nil {
						return nil, fmt.Errorf("%w: group: %s selected: %s, %s", ErrMultipleSameGroup, component.Group, groupSelected.Name, component.Name)
					}

					// Then append to the final list
					selectedComponents = append(selectedComponents, component)
					groupSelected = &component
				}
			}

			// If nothing was selected from a group, handle the default
			if groupSelected == nil && groupDefault != nil {
				selectedComponents = append(selectedComponents, *groupDefault)
			} else if len(groupedComponents[groupKey]) > 1 && groupSelected == nil && groupDefault == nil {
				// If no default component was found, give up
				componentNames := []string{}
				for _, component := range groupedComponents[groupKey] {
					componentNames = append(componentNames, component.Name)
				}
				return nil, fmt.Errorf("%w: choose from %s", ErrNoDefaultOrSelection, strings.Join(componentNames, ", "))
			}
		}

		// Check that we have matched against all requests
		for _, requestedComponent := range f.requestedComponents {
			if _, ok := matchedRequests[requestedComponent]; !ok {
				closeEnough := []string{}
				for _, c := range pkg.Components {
					d := levenshtein.ComputeDistance(c.Name, requestedComponent)
					if d <= 5 {
						closeEnough = append(closeEnough, c.Name)
					}
				}
				return nil, fmt.Errorf("%w: %s, suggestions (%s)", ErrNotFound, requestedComponent, strings.Join(closeEnough, ", "))
			}
		}
	} else {
		for _, groupKey := range orderedComponentGroups {
			group := groupedComponents[groupKey]
			if len(group) > 1 {
				foundDefault := false
				componentNames := []string{}
				for _, component := range group {
					// If the component is default, then use it
					if component.Default {
						selectedComponents = append(selectedComponents, component)
						foundDefault = true
						break
					}
					// Add each component name to the list
					componentNames = append(componentNames, component.Name)
				}
				if !foundDefault {
					// If no default component was found, give up
					return nil, fmt.Errorf("%w: choose from %s", ErrNoDefaultOrSelection, strings.Join(componentNames, ", "))
				}
			} else {
				component := groupedComponents[groupKey][0]

				if !component.Optional {
					selectedComponents = append(selectedComponents, component)
					continue
				}

				if component.Default {
					selectedComponents = append(selectedComponents, component)
					continue
				}
			}
		}
	}

	return selectedComponents, nil
}
