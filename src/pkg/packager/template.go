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
	"github.com/defenseunicorns/zarf/src/types"
)

// fillActiveTemplate handles setting the package templates and reloading the package configuration based on that.
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
				setVal, err := interactive.PromptVariable(types.ZarfPackageVariable{
					Name: key,
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
	err := p.findComponentTemplatesAndReload(&p.cfg.Pkg)
	if err != nil {
		return err
	}

	if err := promptAndSetTemplate("###ZARF_PKG_TMPL_", false); err != nil {
		return err
	}
	// [DEPRECATION] Set the Package Variable syntax as well for backward compatibility
	if err := promptAndSetTemplate("###ZARF_PKG_VAR_", true); err != nil {
		return err
	}

	// Add special variable for the current package architecture
	templateMap["###ZARF_PKG_ARCH###"] = p.arch

	return utils.ReloadYamlTemplate(&p.cfg.Pkg, templateMap)
}

// findComponentTemplatesAndReload appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func (p *Packager) findComponentTemplatesAndReload(config any) error {

	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i, component := range config.(*types.ZarfPackage).Components {
		mappings := map[string]string{}
		mappings["###ZARF_COMPONENT_NAME###"] = component.Name
		err := utils.ReloadYamlTemplate(&config.(*types.ZarfPackage).Components[i], mappings)
		if err != nil {
			return err
		}
	}

	return nil
}
