// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package selection provides interactive CLI component selection.
package selection

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pterm/pterm"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

type deploymentPrompts struct {
	optional func(api.Package, api.Component) (bool, error)
	group    func([]api.Component) (int, error)
}

// DeploymentSelectors prompts for component choices and returns selectors for
// filters.ForDeploy. The caller must supply OS-compatible components and apply
// the returned selectors to the package layout.
func DeploymentSelectors(pkg api.Package) (string, error) {
	return promptDeploymentSelectors(pkg, deploymentPrompts{
		optional: selectOptionalComponent,
		group:    selectChoiceGroup,
	})
}

func promptDeploymentSelectors(pkg api.Package, prompts deploymentPrompts) (string, error) {
	// Preserve v1alpha1 choice groups; v1beta1 components have no Group.
	// Process each group at its first member's position.
	groups := make(map[string][]api.Component)
	var order []string
	for _, component := range pkg.Components {
		key := component.Name
		if component.Group != "" {
			key = component.Group
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], component)
	}

	var selectors []string
	for _, key := range order {
		group := groups[key]
		if len(group) > 1 {
			index, err := prompts.group(group)
			if err != nil {
				return "", fmt.Errorf("selection canceled: %w", err)
			}
			if index < 0 || index >= len(group) {
				return "", fmt.Errorf("selection canceled: invalid component choice %d", index)
			}
			selectors = append(selectors, group[index].Name)
			continue
		}
		component := group[0]
		if !component.Optional {
			continue
		}
		include, err := prompts.optional(pkg, component)
		if err != nil {
			return "", fmt.Errorf("selection canceled: %w", err)
		}
		if include {
			selectors = append(selectors, component.Name)
		} else {
			// Explicitly exclude declined components so v1alpha1 defaults cannot reselect them.
			selectors = append(selectors, "-"+component.Name)
		}
	}
	return filters.NormalizeComponentSelectors(strings.Join(selectors, ","))
}

func selectOptionalComponent(pkg api.Package, component api.Component) (bool, error) {
	message.HorizontalRule()
	definition, err := componentForDisplay(pkg, component)
	if err != nil {
		return false, err
	}
	if err := utils.ColorPrintYAML(definition, nil, false); err != nil {
		return false, err
	}
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Deploy the %s component?", component.Name),
		Default: component.Default,
	}
	var confirm bool
	if err := survey.AskOne(prompt, &confirm); err != nil {
		return false, err
	}
	return confirm, nil
}

func componentForDisplay(pkg api.Package, component api.Component) (any, error) {
	displayPackage := api.Package{APIVersion: pkg.APIVersion, Components: []api.Component{component}}
	switch pkg.GetAPIVersion() {
	case v1alpha1.APIVersion:
		return convert.PackageToV1alpha1(displayPackage).Components[0], nil
	case v1beta1.APIVersion:
		return convert.PackageToV1beta1(displayPackage).Components[0], nil
	default:
		return nil, fmt.Errorf("unsupported package apiVersion %q", pkg.GetAPIVersion())
	}
}

func selectChoiceGroup(componentGroup []api.Component) (int, error) {
	message.HorizontalRule()
	options := make([]string, 0, len(componentGroup))
	for _, component := range componentGroup {
		options = append(options, fmt.Sprintf("Name: %s\n  Description: %s\n", component.Name, component.Description))
	}
	prompt := &survey.Select{Message: "Select a component to deploy:", Options: options}
	pterm.Println()
	var chosen int
	err := survey.AskOne(prompt, &chosen)
	return chosen, err
}
