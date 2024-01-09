// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/agnivade/levenshtein"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

type selectState int

const (
	unknown selectState = iota
	included
	excluded
)

// GetSelectedComponents prompts to select components based upon multiple conditions
func GetSelectedComponents(optionalComponents string, allComponents []types.ZarfComponent) ([]types.ZarfComponent, error) {
	var selectedComponents []types.ZarfComponent
	groupedComponents := map[string][]types.ZarfComponent{}
	orderedComponentGroups := []string{}

	// Group the components by Name and Group while maintaining order
	for _, component := range allComponents {
		groupKey := component.Name
		if component.DeprecatedGroup != "" {
			groupKey = component.DeprecatedGroup
		}

		if !slices.Contains(orderedComponentGroups, groupKey) {
			orderedComponentGroups = append(orderedComponentGroups, groupKey)
		}

		groupedComponents[groupKey] = append(groupedComponents[groupKey], component)
	}

	// Split the --components list as a comma-delimited list
	requestedComponents := helpers.StringToSlice(optionalComponents)
	isPartial := len(requestedComponents) > 0 && requestedComponents[0] != ""

	if isPartial {
		matchedRequests := map[string]bool{}

		// NOTE: This does not use forIncludedComponents as it takes group, default and required status into account.
		for _, groupKey := range orderedComponentGroups {
			var groupDefault *types.ZarfComponent
			var groupSelected *types.ZarfComponent

			for _, component := range groupedComponents[groupKey] {
				// Ensure we have a local version of the component to point to (otherwise the pointer might change on us)
				component := component

				selectState, matchedRequest := includedOrExcluded(component, requestedComponents)

				if !component.IsRequired() {
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
		var err error
		for _, requestedComponent := range requestedComponents {
			if _, ok := matchedRequests[requestedComponent]; !ok {
				closeEnough := []string{}
				for _, c := range allComponents {
					d := levenshtein.ComputeDistance(c.Name, requestedComponent)
					if d <= 5 {
						closeEnough = append(closeEnough, c.Name)
					}
				}
				failure := fmt.Errorf(lang.PkgDeployErrNoCompatibleComponentsForSelection, requestedComponent, strings.Join(closeEnough, ", "))
				if err != nil {
					err = fmt.Errorf("%w, %w", err, failure)
				} else {
					err = failure
				}
			}
		}
		if err != nil {
			return []types.ZarfComponent{}, err
		}
	} else {
		for _, groupKey := range orderedComponentGroups {
			if len(groupedComponents[groupKey]) > 1 {
				component := SelectChoiceGroup(groupedComponents[groupKey])
				selectedComponents = append(selectedComponents, component)
			} else {
				component := groupedComponents[groupKey][0]

				if component.IsRequired() {
					selectedComponents = append(selectedComponents, component)
				} else if selected := SelectOptionalComponent(component); selected {
					selectedComponents = append(selectedComponents, component)
				}
			}
		}
	}

	return selectedComponents, nil
}

// GetOnlyIncludedComponents returns only the components that are included
func GetOnlyIncludedComponents(optionalComponents string, components []types.ZarfComponent) ([]types.ZarfComponent, error) {
	requestedComponents := helpers.StringToSlice(optionalComponents)
	isPartial := len(requestedComponents) > 0 && requestedComponents[0] != ""

	result := []types.ZarfComponent{}

	for _, component := range components {
		selectState := unknown

		if isPartial {
			selectState, _ = includedOrExcluded(component, requestedComponents)

			if selectState == excluded {
				continue
			}
		} else {
			selectState = included
		}

		if selectState == included {
			result = append(result, component)
		}
	}

	return result, nil
}

// ForIncludedComponents runs a function for each included component
func ForIncludedComponents(optionalComponents string, components []types.ZarfComponent, fn func(types.ZarfComponent) error) error {
	included, err := GetOnlyIncludedComponents(optionalComponents, components)
	if err != nil {
		return err
	}

	for _, component := range included {
		if err := fn(component); err != nil {
			return err
		}
	}

	return nil
}

func includedOrExcluded(component types.ZarfComponent, requestedComponentNames []string) (selectState, string) {
	// Check if the component has a leading dash indicating it should be excluded - this is done first so that exclusions precede inclusions
	for _, requestedComponent := range requestedComponentNames {
		if strings.HasPrefix(requestedComponent, "-") {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(strings.TrimPrefix(requestedComponent, "-"), component.Name); matched {
				return excluded, requestedComponent
			}
		}
	}
	// Check if the component matches a glob pattern and should be included
	for _, requestedComponent := range requestedComponentNames {
		// If the component glob matches one of the requested components, then return true
		// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
		if matched, _ := path.Match(requestedComponent, component.Name); matched {
			return included, requestedComponent
		}
	}

	// All other cases we don't know if we should include or exclude yet
	return unknown, ""
}
