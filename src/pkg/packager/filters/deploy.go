// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
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

// Apply applies the filter.
func (f *deploymentFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	var selectedComponents []types.ZarfComponent
	groupedComponents := map[string][]types.ZarfComponent{}
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
			var groupDefault *types.ZarfComponent
			var groupSelected *types.ZarfComponent

			for _, component := range groupedComponents[groupKey] {
				// Ensure we have a local version of the component to point to (otherwise the pointer might change on us)
				component := component

				selectState, matchedRequest := includedOrExcluded(component.Name, f.requestedComponents)

				if !isRequired(component) {
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
						return []types.ZarfComponent{}, fmt.Errorf(lang.PkgDeployErrMultipleComponentsSameGroup, groupSelected.Name, component.Name, component.DeprecatedGroup)
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
				return []types.ZarfComponent{}, fmt.Errorf(lang.PkgDeployErrNoDefaultOrSelection, strings.Join(componentNames, ", "))
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
				return nil, fmt.Errorf(lang.PkgDeployErrNoCompatibleComponentsForSelection, requestedComponent, strings.Join(closeEnough, ", "))
			}
		}
	} else {
		for _, groupKey := range orderedComponentGroups {
			group := groupedComponents[groupKey]
			if len(group) > 1 {
				if f.isInteractive {
					component := interactive.SelectChoiceGroup(group)
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
						return []types.ZarfComponent{}, fmt.Errorf(lang.PkgDeployErrNoDefaultOrSelection, strings.Join(componentNames, ", "))
					}
				}
			} else {
				component := groupedComponents[groupKey][0]

				// default takes precedence over required
				if component.Default {
					selectedComponents = append(selectedComponents, component)
					continue
				}

				// otherwise interactively prompt the user
				if f.isInteractive {
					selected, err := interactive.SelectOptionalComponent(component)
					if err != nil {
						return []types.ZarfComponent{}, fmt.Errorf(lang.PkgDeployErrComponentSelectionCanceled, err.Error())
					}
					if selected {
						selectedComponents = append(selectedComponents, component)
						continue
					}
				}

				// finally go off the required status
				if isRequired(component) {
					selectedComponents = append(selectedComponents, component)
					continue
				}
			}
		}
	}

	return selectedComponents, nil
}
