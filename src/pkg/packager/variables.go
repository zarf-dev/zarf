// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// fillActiveTemplate handles setting the active variables and reloading the base template.
func (p *Packager) fillActiveTemplate() error {
	templateMap := map[string]string{}

	promptAndSetTemplate := func(templatePrefix string, deprecated bool) error {
		// Ensure uppercase keys
		setFromCLIConfig := utils.TransformMapKeys(p.cfg.CreateOpts.SetVariables, strings.ToUpper)

		yamlTemplates, err := utils.FindYamlTemplates(&p.cfg.Pkg, templatePrefix, "###")
		if err != nil {
			return err
		}

		for key := range yamlTemplates {
			if deprecated {
				message.Warnf(lang.PkgValidateTemplateDeprecation, key, key, key)
			}

			_, present := setFromCLIConfig[key]
			if !present && !config.CommonOptions.Confirm {
				setVal, err := p.promptVariable(types.ZarfPackageVariable{
					Name: key,
				})

				if err == nil {
					setFromCLIConfig[key] = setVal
				} else {
					return err
				}
			} else if !present {
				return fmt.Errorf("template '%s' must be '--set' when using the '--confirm' flag", key)
			}
		}

		for key, value := range setFromCLIConfig {
			templateMap[fmt.Sprintf("%s%s###", templatePrefix, key)] = value
		}

		return nil
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

// setVariableMapInConfig handles setting the active variables used to template component files.
func (p *Packager) setVariableMapInConfig() error {
	// Ensure uppercase keys
	setVariableValues := utils.TransformMapKeys(p.cfg.DeployOpts.SetVariables, strings.ToUpper)
	for name, value := range setVariableValues {
		p.setVariableInConfig(name, value, false, false)
	}

	for _, variable := range p.cfg.Pkg.Variables {
		_, present := p.cfg.SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			p.cfg.SetVariableMap[variable.Name].Sensitive = variable.Sensitive
			p.cfg.SetVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			continue
		}

		// First set default (may be overridden by prompt)
		p.setVariableInConfig(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := p.promptVariable(variable)

			if err != nil {
				return err
			}

			p.setVariableInConfig(variable.Name, val, variable.Sensitive, variable.AutoIndent)
		}
	}

	return nil
}

func (p *Packager) setVariableInConfig(name, value string, sensitive bool, autoIndent bool) {
	p.cfg.SetVariableMap[name] = &types.ZarfSetVariable{
		Name:       name,
		Value:      value,
		Sensitive:  sensitive,
		AutoIndent: autoIndent,
	}
}

// injectImportedVariable determines if an imported package variable exists in the active config and adds it if not.
func (p *Packager) injectImportedVariable(importedVariable types.ZarfPackageVariable) {
	presentInActive := false
	for _, configVariable := range p.cfg.Pkg.Variables {
		if configVariable.Name == importedVariable.Name {
			presentInActive = true
		}
	}

	if !presentInActive {
		p.cfg.Pkg.Variables = append(p.cfg.Pkg.Variables, importedVariable)
	}
}

// injectImportedConstant determines if an imported package constant exists in the active config and adds it if not.
func (p *Packager) injectImportedConstant(importedConstant types.ZarfPackageConstant) {
	presentInActive := false
	for _, configVariable := range p.cfg.Pkg.Constants {
		if configVariable.Name == importedConstant.Name {
			presentInActive = true
		}
	}

	if !presentInActive {
		p.cfg.Pkg.Constants = append(p.cfg.Pkg.Constants, importedConstant)
	}
}
