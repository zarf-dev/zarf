// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/agnivade/levenshtein"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	_ ComponentFilterStrategy = &DeploymentFilter{}
)

// NewDeploymentFilter creates a new deployment filter.
func NewDeploymentFilter(optionalComponents string) *DeploymentFilter {
	requested := helpers.StringToSlice(optionalComponents)

	return &DeploymentFilter{
		requested,
	}
}

// DeploymentFilter is the default filter for deployments.
type DeploymentFilter struct {
	requestedComponents []string
}

// Apply applies the filter.
func (f *DeploymentFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	useRequiredLogic := false
	if pkg.Build.Version != config.UnsetCLIVersion {
		buildVersion, err := semver.NewVersion(pkg.Build.Version)
		if err != nil {
			return []types.ZarfComponent{}, fmt.Errorf("unable to parse package version: %w", err)
		}
		if buildVersion.LessThan(semver.MustParse("v0.33.0")) {
			useRequiredLogic = true
		}
	}

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

				if !isRequired(component, useRequiredLogic) {
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
		for _, requestedComponent := range f.requestedComponents {
			if _, ok := matchedRequests[requestedComponent]; !ok {
				closeEnough := []string{}
				for _, c := range pkg.Components {
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
				component := interactive.SelectChoiceGroup(groupedComponents[groupKey])
				selectedComponents = append(selectedComponents, component)
			} else {
				component := groupedComponents[groupKey][0]

				if isRequired(component, useRequiredLogic) {
					selectedComponents = append(selectedComponents, component)
				} else if selected := interactive.SelectOptionalComponent(component); selected {
					selectedComponents = append(selectedComponents, component)
				}
			}
		}
	}

	return selectedComponents, nil
}
