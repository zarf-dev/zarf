// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/agnivade/levenshtein"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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
func (f *deploymentFilter) Apply(pkg PackageView) ([]int, error) {
	var selectedComponents []int
	groupedComponents := map[string][]indexedComponent{}
	orderedComponentGroups := []string{}

	// Group the components by Name and Group while maintaining order
	for idx, component := range pkg.Components {
		groupKey := component.Name
		if component.Group != "" {
			groupKey = component.Group
		}

		if !slices.Contains(orderedComponentGroups, groupKey) {
			orderedComponentGroups = append(orderedComponentGroups, groupKey)
		}

		groupedComponents[groupKey] = append(groupedComponents[groupKey], indexedComponent{idx: idx, component: component})
	}

	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""

	if isPartial {
		matchedRequests := map[string]bool{}

		// NOTE: This does not use forIncludedComponents as it takes group, default and required status into account.
		for _, groupKey := range orderedComponentGroups {
			var groupDefault *indexedComponent
			var groupSelected *indexedComponent

			for _, component := range groupedComponents[groupKey] {
				// Ensure we have a local version of the component to point to (otherwise the pointer might change on us)
				component := component

				selectState, matchedRequest, err := includedOrExcluded(component.component.Name, f.requestedComponents)
				if err != nil {
					return nil, err
				}

				if component.component.Optional {
					if selectState == excluded {
						// If the component was explicitly excluded, record the match and continue
						matchedRequests[matchedRequest] = true
						continue
					} else if selectState == unknown && component.component.Default && groupDefault == nil {
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
						return nil, fmt.Errorf("%w: group: %s selected: %s, %s", ErrMultipleSameGroup, component.component.Group, groupSelected.component.Name, component.component.Name)
					}

					// Then append to the final list
					selectedComponents = append(selectedComponents, component.idx)
					groupSelected = &component
				}
			}

			// If nothing was selected from a group, handle the default
			if groupSelected == nil && groupDefault != nil {
				selectedComponents = append(selectedComponents, groupDefault.idx)
			} else if len(groupedComponents[groupKey]) > 1 && groupSelected == nil && groupDefault == nil {
				// If no default component was found, give up
				componentNames := []string{}
				for _, component := range groupedComponents[groupKey] {
					componentNames = append(componentNames, component.component.Name)
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
					component, err := selectChoiceGroup(group)
					if err != nil {
						return nil, fmt.Errorf("%w: %w", ErrSelectionCanceled, err)
					}
					selectedComponents = append(selectedComponents, component.idx)
				} else {
					foundDefault := false
					componentNames := []string{}
					for _, component := range group {
						// If the component is default, then use it
						if component.component.Default {
							selectedComponents = append(selectedComponents, component.idx)
							foundDefault = true
							break
						}
						// Add each component name to the list
						componentNames = append(componentNames, component.component.Name)
					}
					if !foundDefault {
						// If no default component was found, give up
						return nil, fmt.Errorf("%w: choose from %s", ErrNoDefaultOrSelection, strings.Join(componentNames, ", "))
					}
				}
			} else {
				component := groupedComponents[groupKey][0]

				if !component.component.Optional {
					selectedComponents = append(selectedComponents, component.idx)
					continue
				}

				if f.isInteractive {
					selected, err := selectOptionalComponent(component.component)
					if err != nil {
						return nil, fmt.Errorf("%w: %w", ErrSelectionCanceled, err)
					}
					if selected {
						selectedComponents = append(selectedComponents, component.idx)
						continue
					}
				}

				if component.component.Default {
					selectedComponents = append(selectedComponents, component.idx)
					continue
				}
			}
		}
	}

	return selectedComponents, nil
}

type indexedComponent struct {
	idx       int
	component ComponentView
}

func selectOptionalComponent(component ComponentView) (bool, error) {
	message.HorizontalRule()

	definition := component.Definition
	if definition == nil {
		definition = component
	}
	err := utils.ColorPrintYAML(definition, nil, false)
	if err != nil {
		return false, err
	}

	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Deploy the %s component?", component.Name),
		Default: component.Default,
	}

	var confirm bool
	err = survey.AskOne(prompt, &confirm)
	if err != nil {
		return false, err
	}
	return confirm, nil
}

func selectChoiceGroup(componentGroup []indexedComponent) (indexedComponent, error) {
	message.HorizontalRule()

	var chosen int
	options := make([]string, 0, len(componentGroup))

	for _, component := range componentGroup {
		text := fmt.Sprintf("Name: %s\n  Description: %s\n", component.component.Name, component.component.Description)
		options = append(options, text)
	}

	prompt := &survey.Select{
		Message: "Select a component to deploy:",
		Options: options,
	}

	pterm.Println()

	return componentGroup[chosen], survey.AskOne(prompt, &chosen)
}
