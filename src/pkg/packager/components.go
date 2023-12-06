// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

func (p *Packager) getValidComponents() []types.ZarfComponent {
	var validComponentsList []types.ZarfComponent
	var orderedKeys []string
	var choiceComponents []string

	componentGroups := make(map[string][]types.ZarfComponent)

	// The component list is comma-delimited list
	requestedComponents := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)

	// Break up components into choice groups
	for _, component := range p.cfg.Pkg.Components {
		matchFn := func(a, b string) bool { return a == b }
		key := component.Group
		// If not a choice group, then use the component name as the key
		if key == "" {
			key = component.Name
		} else {
			// Otherwise, add the component name to the choice group list for later validation
			choiceComponents = helpers.MergeSlices(choiceComponents, []string{component.Name}, matchFn)
		}

		// Preserve component order
		orderedKeys = helpers.MergeSlices(orderedKeys, []string{key}, matchFn)

		// Append the component to the list of components in the group
		componentGroups[key] = append(componentGroups[key], component)
	}

	// Loop through each component group in original order and handle required, requested or user confirmation
	for _, key := range orderedKeys {

		componentGroup := componentGroups[key]

		// Choice groups are handled differently for user confirmation
		userChoicePrompt := len(componentGroup) > 1

		// Loop through the components in the group
		for _, component := range componentGroup {
			included, excluded := false, false

			// If the component is required, then it is always included
			if component.Required {
				included = true
			} else {
				// First check if the component is required or requested via CLI flag
				included, excluded = includedOrExcluded(component, requestedComponents)

				if excluded {
					continue
				}
			}

			// If the user has not requested this component via CLI flag, then prompt them if not a choice group
			if !included && !userChoicePrompt {
				included = confirmOptionalComponent(component)
			}

			if included {
				// Mark deployment as appliance mode if this is an init config and the k3s component is enabled
				if component.Name == k8s.DistroIsK3s && p.isInitConfig() {
					p.cfg.InitOpts.ApplianceMode = true
				}
				// Add the component to the list of valid components
				validComponentsList = append(validComponentsList, component)
				// Ensure that the component is not requested again if in a choice group
				userChoicePrompt = false
				// Exit the inner loop on a match since groups should only have one requested component
				break
			}
		}

		// If the user has requested a choice group, then prompt them
		if userChoicePrompt {
			selectedComponent := confirmChoiceGroup(componentGroup)
			validComponentsList = append(validComponentsList, selectedComponent)
		}
	}

	// Ensure all user requested components are valid
	if err := validateRequests(validComponentsList, requestedComponents, choiceComponents); err != nil {
		message.Fatalf(err, "Invalid component argument, %s", err)
	}

	return validComponentsList
}

func (p *Packager) forRequestedComponents(function func(types.ZarfComponent) error) error {
	requestedComponents := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)
	partialMirror := len(requestedComponents) > 0 && requestedComponents[0] != ""

	for _, component := range p.cfg.Pkg.Components {
		included, excluded := false, false

		if partialMirror {
			included, excluded = includedOrExcluded(component, requestedComponents)

			if excluded {
				continue
			}
		} else {
			included = true
		}

		if included {
			if err := function(component); err != nil {
				return err
			}
		}
	}

	return nil
}

// Match on the first requested component that is not in the list of valid components and return the component name.
func validateRequests(validComponentsList []types.ZarfComponent, requestedComponentNames, choiceComponents []string) error {
	// Loop through each requested component names
	for _, requestedComponent := range requestedComponentNames {
		if strings.HasSuffix(requestedComponent, "-") {
			continue
		}

		found := false
		// Match on the first requested component that is a valid component
		for _, component := range validComponentsList {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(requestedComponent, component.Name); matched {
				found = true
				break
			}
		}

		// If the requested component was not found, then return an error
		if !found {
			// If the requested component is in a choice group, then warn the user they must choose only one
			for _, component := range choiceComponents {
				if component == requestedComponent {
					return fmt.Errorf("component %s is part of a group of components and only one may be chosen", requestedComponent)
				}
			}
			// Otherwise, return an error a general error
			return fmt.Errorf("unable to find component %s", requestedComponent)
		}
	}

	return nil
}

func includedOrExcluded(component types.ZarfComponent, requestedComponentNames []string) (include bool, exclude bool) {
	// Otherwise,check if this is one of the components that has been requested from the CLI
	for _, requestedComponent := range requestedComponentNames {
		// Check if the component has a trailing dash indicating it should be excluded
		if strings.HasSuffix(requestedComponent, "-") {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(strings.TrimSuffix(requestedComponent, "-"), component.Name); matched {
				return false, true
			}
		} else {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			if matched, _ := path.Match(requestedComponent, component.Name); matched {
				return true, false
			}
		}
	}

	// All other cases we don't know if we should include or exclude yet
	return false, false
}

// Confirm optional component.
func confirmOptionalComponent(component types.ZarfComponent) (confirmComponent bool) {
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

	// Since no requested components were provided, prompt the user
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Deploy the %s component?", component.Name),
		Default: component.Default,
	}
	if err := survey.AskOne(prompt, &confirmComponent); err != nil {
		message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
	}
	return confirmComponent
}

func confirmChoiceGroup(componentGroup []types.ZarfComponent) types.ZarfComponent {
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
		message.Fatalf(nil, "You must specify at least one component from the group %#v when using the --confirm flag.", componentNames)
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
		message.Fatalf(nil, "Component selection canceled: %s", err.Error())
	}

	return componentGroup[chosen]
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
