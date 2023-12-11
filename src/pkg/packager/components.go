// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"path"
	"slices"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

const (
	unknown  = 0
	included = 1
	excluded = 2
)

func (p *Packager) getSelectedComponents() []types.ZarfComponent {
	var selectedComponents []types.ZarfComponent
	groupedComponents := map[string][]types.ZarfComponent{}
	orderedComponentGroups := []string{}

	// Group the components by Name and Group while maintaining order
	for _, component := range p.cfg.Pkg.Components {
		groupKey := component.Name
		if component.Group != "" {
			groupKey = component.Group
		}

		if !slices.Contains(orderedComponentGroups, groupKey) {
			orderedComponentGroups = append(orderedComponentGroups, groupKey)
		}

		groupedComponents[groupKey] = append(groupedComponents[groupKey], component)
	}

	// Split the --components list as a comma-delimited list
	requestedComponents := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)
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

				if !component.Required {
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
						message.Fatalf(nil, "You cannot specify multiple components (%q, %q) within the same group (%q) when using the --components flag.", groupSelected.Name, component.Name, component.Group)
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
				message.Fatalf(nil, "You must make a selection from %q with the --components flag as there is no default in their group.", strings.Join(componentNames, ","))
			}
		}

		// Check that we have matched against all requests
		for _, requestedComponent := range requestedComponents {
			if _, ok := matchedRequests[requestedComponent]; !ok {
				message.Fatalf(nil, "No compatible components found that matched %q. Please check spelling and try again.", requestedComponent)
			}
		}
	} else {
		for _, groupKey := range orderedComponentGroups {
			if len(groupedComponents[groupKey]) > 1 {
				component := interactive.SelectChoiceGroup(groupedComponents[groupKey])
				selectedComponents = append(selectedComponents, component)
			} else {
				component := groupedComponents[groupKey][0]

				if component.Required {
					selectedComponents = append(selectedComponents, component)
				} else if selected := interactive.SelectOptionalComponent(component); selected {
					selectedComponents = append(selectedComponents, component)
				}
			}
		}
	}

	return selectedComponents
}

func (p *Packager) forIncludedComponents(onIncluded func(types.ZarfComponent) error) error {
	requestedComponents := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)
	isPartial := len(requestedComponents) > 0 && requestedComponents[0] != ""

	for _, component := range p.cfg.Pkg.Components {
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
			if err := onIncluded(component); err != nil {
				return err
			}
		}
	}

	return nil
}

func includedOrExcluded(component types.ZarfComponent, requestedComponentNames []string) (int, string) {
	// Check if this is one of the components that has been specified from the CLI
	for _, requestedComponent := range requestedComponentNames {
		// Check if the component has a trailing dash indicating it should be excluded
		if strings.HasSuffix(requestedComponent, "-") {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(strings.TrimSuffix(requestedComponent, "-"), component.Name); matched {
				return excluded, requestedComponent
			}
		} else {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(requestedComponent, component.Name); matched {
				return included, requestedComponent
			}
		}
	}

	// All other cases we don't know if we should include or exclude yet
	return unknown, ""
}

func requiresCluster(component types.ZarfComponent) bool {
	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	if hasImages || hasCharts || hasManifests || hasRepos || hasDataInjections {
		return true
	}

	return false
}
