// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

type selectState int

const (
	unknown selectState = iota
	included
	excluded
)

func GetSelectedComponents(optionalComponents string, allComponents []types.ZarfComponent) []types.ZarfComponent {
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
						message.Fatalf(nil, lang.PkgDeployErrMultipleComponentsSameGroup, groupSelected.Name, component.Name, component.DeprecatedGroup)
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
				message.Fatalf(nil, lang.PkgDeployErrNoDefaultOrSelection, strings.Join(componentNames, ","))
			}
		}

		// Check that we have matched against all requests
		for _, requestedComponent := range requestedComponents {
			if _, ok := matchedRequests[requestedComponent]; !ok {
				message.Fatalf(nil, lang.PkgDeployErrNoCompatibleComponentsForSelection, requestedComponent)
			}
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

	return selectedComponents
}

func ForIncludedComponents(optionalComponents string, components []types.ZarfComponent, onIncluded func(types.ZarfComponent) error) error {
	requestedComponents := helpers.StringToSlice(optionalComponents)
	isPartial := len(requestedComponents) > 0 && requestedComponents[0] != ""

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
			if err := onIncluded(component); err != nil {
				return err
			}
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

// SelectOptionalComponent prompts to confirm optional components
func SelectOptionalComponent(component types.ZarfComponent) (confirmComponent bool) {
	// Confirm flag passed, just use defaults
	if config.CommonOptions.Confirm {
		return component.Default
	}

	message.HorizontalRule()

	displayComponent := component
	displayComponent.Description = ""
	utils.ColorPrintYAML(displayComponent, nil, false)
	if component.Description != "" {
		message.Question(component.Description)
	}

	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Deploy the %s component?", component.Name),
		Default: component.Default,
	}
	if err := survey.AskOne(prompt, &confirmComponent); err != nil {
		message.Fatalf(nil, lang.PkgDeployErrComponentSelectionCanceled, err.Error())
	}

	return confirmComponent
}

// SelectChoiceGroup prompts to select component groups
func SelectChoiceGroup(componentGroup []types.ZarfComponent) types.ZarfComponent {
	// Confirm flag passed, just use defaults
	if config.CommonOptions.Confirm {
		var componentNames []string
		for _, component := range componentGroup {
			// If the component is default, then return it
			if component.Default {
				return component
			}
			// Add each component name to the list
			componentNames = append(componentNames, component.Name)
		}
		// If no default component was found, give up
		message.Fatalf(nil, lang.PkgDeployErrNoDefaultOrSelection, strings.Join(componentNames, ","))
	}

	message.HorizontalRule()

	var chosen int
	var options []string

	for _, component := range componentGroup {
		text := fmt.Sprintf("Name: %s\n  Description: %s\n", component.Name, component.Description)
		options = append(options, text)
	}

	prompt := &survey.Select{
		Message: "Select a component to deploy:",
		Options: options,
	}

	pterm.Println()

	if err := survey.AskOne(prompt, &chosen); err != nil {
		message.Fatalf(nil, lang.PkgDeployErrComponentSelectionCanceled, err.Error())
	}

	return componentGroup[chosen]
}
