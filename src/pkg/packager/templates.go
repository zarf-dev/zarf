// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
)

// ReloadComponentTemplate appends ###ZARF_COMPONENT_NAME### for the component, assigns value, and reloads
// Any instance of ###ZARF_COMPONENT_NAME### within a component will be replaced with that components name
func ReloadComponentTemplate(component *types.ZarfComponent) error {
	mappings := map[string]string{}
	mappings[types.ZarfComponentName] = component.Name
	err := utils.ReloadYamlTemplate(component, mappings)
	if err != nil {
		return err
	}
	return nil
}

// ReloadComponentTemplatesInPackage appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func ReloadComponentTemplatesInPackage(zarfPackage *types.ZarfPackage) error {
	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i := range zarfPackage.Components {
		if err := ReloadComponentTemplate(&zarfPackage.Components[i]); err != nil {
			return err
		}
	}

	return nil
}

// fillActiveTemplate handles setting the active variables and reloading the base template.
func (p *Packager) fillActiveTemplate() error {
	templateMap := map[string]string{}

	promptAndSetTemplate := func(templatePrefix string, deprecated bool) error {
		yamlTemplates, err := utils.FindYamlTemplates(&p.cfg.Pkg, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				p.warnings = append(p.warnings, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
			}

			_, present := p.cfg.CreateOpts.SetVariables[key]
			if !present && !config.CommonOptions.Confirm {
				setVal, err := interactive.PromptVariable(variables.InteractiveVariable{
					Variable: variables.Variable{Name: key},
				})

				if err == nil {
					p.cfg.CreateOpts.SetVariables[key] = setVal
				} else {
					return err
				}
			} else if !present {
				return fmt.Errorf("template '%s' must be '--set' when using the '--confirm' flag", key)
			}
		}

		for key, value := range p.cfg.CreateOpts.SetVariables {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}

		return nil
	}

	// update the component templates on the package
	err := ReloadComponentTemplatesInPackage(&p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err := promptAndSetTemplate(types.ZarfPackageTemplatePrefix, false); err != nil {
		return err
	}
	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := promptAndSetTemplate(types.ZarfPackageVariablePrefix, true); err != nil {
		return err
	}

	// Add special variable for the current package architecture
	templateMap[types.ZarfPackageArch] = p.arch

	return utils.ReloadYamlTemplate(&p.cfg.Pkg, templateMap)
}
