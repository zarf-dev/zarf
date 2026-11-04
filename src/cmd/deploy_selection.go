// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"fmt"
	"runtime"
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

type filterableLayout interface {
	Filter(filters.ComponentFilterStrategy) error
}

func resolveInteractiveDeploymentSelectors(pkgLayout filterableLayout, optionalComponents string) (string, error) {
	return resolveDeploymentSelectors(pkgLayout, optionalComponents, deploymentPrompts{
		optional: selectOptionalComponent,
		group:    selectChoiceGroup,
	})
}

func resolveDeploymentSelectors(pkgLayout filterableLayout, optionalComponents string, prompts deploymentPrompts) (string, error) {
	normalized, err := filters.NormalizeComponentSelectors(optionalComponents)
	if err != nil {
		return "", err
	}
	if normalized != "" {
		if err := pkgLayout.Filter(filters.Combine(filters.ByLocalOS(runtime.GOOS), filters.ForDeploy(normalized))); err != nil {
			return "", err
		}
		return normalized, nil
	}

	selection := &interactiveDeploymentFilter{prompts: prompts}
	if err := pkgLayout.Filter(filters.Combine(filters.ByLocalOS(runtime.GOOS), selection)); err != nil {
		return "", err
	}
	return filters.NormalizeComponentSelectors(strings.Join(selection.selectors, ","))
}

type interactiveDeploymentFilter struct {
	prompts   deploymentPrompts
	selectors []string
}

func (f *interactiveDeploymentFilter) Apply(pkg api.Package) ([]api.Component, error) {
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

	var selected []api.Component
	var selectors []string
	for _, key := range order {
		group := groups[key]
		if len(group) > 1 {
			index, err := f.prompts.group(group)
			if err != nil {
				return nil, fmt.Errorf("selection canceled: %w", err)
			}
			if index < 0 || index >= len(group) {
				return nil, fmt.Errorf("selection canceled: invalid component choice %d", index)
			}
			selected = append(selected, group[index])
			selectors = append(selectors, group[index].Name)
			continue
		}
		component := group[0]
		if !component.Optional {
			selected = append(selected, component)
			continue
		}
		include, err := f.prompts.optional(pkg, component)
		if err != nil {
			return nil, fmt.Errorf("selection canceled: %w", err)
		}
		if include {
			selected = append(selected, component)
			selectors = append(selectors, component.Name)
		} else {
			selectors = append(selectors, "-"+component.Name)
		}
	}
	f.selectors = selectors
	return selected, nil
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
