// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// ConfirmOptionalComponent prompts to confirm optional components
func ConfirmOptionalComponent(component types.ZarfComponent) (confirmComponent bool) {
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

// ConfirmChoiceGroup prompts to select component groups
func ConfirmChoiceGroup(componentGroup []types.ZarfComponent) types.ZarfComponent {
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
