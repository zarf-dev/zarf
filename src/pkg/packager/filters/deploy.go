// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
)

// ForDeploy creates a new deployment filter.
func ForDeploy(optionalComponents string, isInteractive bool) ComponentFilterStrategy {
	requested := helpers.StringToSlice(optionalComponents)

	return &deploymentFilter{
		requested,
		isInteractive,
	}
}

// deploymentFilter is the default filter for deployments.
type deploymentFilter struct {
	requestedComponents []string
	isInteractive       bool
}

// Errors for the deployment filter.
var (
	ErrMultipleSameGroup    = fmt.Errorf("cannot specify multiple components from the same group")
	ErrNoDefaultOrSelection = fmt.Errorf("no default or selected component found")
	ErrNotFound             = fmt.Errorf("no compatible components found")
	ErrSelectionCanceled    = fmt.Errorf("selection canceled")
)

// Apply applies the filter.
func (f *deploymentFilter) Apply(pkg v1beta1.ZarfPackage) ([]v1beta1.ZarfComponent, error) {
	var selectedComponents []v1beta1.ZarfComponent
	groupedComponents := map[string][]v1beta1.ZarfComponent{}
	orderedComponentGroups := []string{}

	// Group the components by Name and Group while maintaining order
	for _, component := range pkg.Components {
		groupKey := component.Name
		if component.DeprecatedGroup != "" {
			groupKey = component.DeprecatedGroup
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
			var groupDefault *v1beta1.ZarfComponent
			var groupSelected *v1beta1.ZarfComponent

			for _, component := range groupedComponents[groupKey] {
				// Ensure we have a local version of the component to point to (otherwise the pointer might change on us)
				component := component

				selectState, matchedRequest := includedOrExcluded(component.Name, f.requestedComponents)

				if component.IsOptional() {
					if selectState == excluded {
						// If the component was explicitly excluded, record the match and continue
						matchedRequests[matchedRequest] = true
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
					// If the component was explicitly included, record the match
					matchedRequests[matchedRequest] = true

					// Then check for already selected groups
					if groupSelected != nil {
						return nil, fmt.Errorf("%w: group: %s selected: %s, %s", ErrMultipleSameGroup, component.DeprecatedGroup, groupSelected.Name, component.Name)
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
				if f.isInteractive {
					component, err := interactive.SelectChoiceGroup(group)
					if err != nil {
						return nil, fmt.Errorf("%w: %w", ErrSelectionCanceled, err)
					}
					selectedComponents = append(selectedComponents, component)
				} else {
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
				}
			} else {
				component := groupedComponents[groupKey][0]

				if !component.IsOptional() {
					selectedComponents = append(selectedComponents, component)
					continue
				}

				if f.isInteractive {
					selected, err := interactive.SelectOptionalComponent(component)
					if err != nil {
						return nil, fmt.Errorf("%w: %w", ErrSelectionCanceled, err)
					}
					if selected {
						selectedComponents = append(selectedComponents, component)
						continue
					}
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
