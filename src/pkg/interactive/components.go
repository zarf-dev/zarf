// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// SelectOptionalComponent prompts to confirm optional components
func SelectOptionalComponent(component v1alpha1.ZarfComponent) (bool, error) {
	message.HorizontalRule()

	displayComponent := component
	displayComponent.Description = ""
	err := utils.ColorPrintYAML(displayComponent, nil, false)
	if err != nil {
		return false, err
	}
	if component.Description != "" {
		// TODO (@austinabro321) once we move interactiveness to CLI level we should replace this with logger.Info
		message.Question(component.Description)
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

// SelectChoiceGroup prompts to select component groups
func SelectChoiceGroup(componentGroup []v1alpha1.ZarfComponent) (v1alpha1.ZarfComponent, error) {
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

	return componentGroup[chosen], survey.AskOne(prompt, &chosen)
}
