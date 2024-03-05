// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package interactive contains functions for interacting with the user via STDIN.
package interactive

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// SelectOptionalComponent prompts to confirm optional components
func SelectOptionalComponent(component types.ZarfComponent) (confirm bool, err error) {
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

	return confirm, survey.AskOne(prompt, &confirm)
}

// SelectChoiceGroup prompts to select component groups
func SelectChoiceGroup(componentGroup []types.ZarfComponent) types.ZarfComponent {
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
