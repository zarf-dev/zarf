// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

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
	err := p.findComponentTemplatesAndReload()
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

// setVariableMapInConfig handles setting the active variables used to template component files.
func (p *Packager) setVariableMapInConfig() error {
	for name, value := range p.cfg.PkgOpts.SetVariables {
		p.setVariableInConfig(name, value, false, false, "")
	}

	for _, variable := range p.cfg.Pkg.Variables {
		_, present := p.cfg.SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			p.cfg.SetVariableMap[variable.Name].Sensitive = variable.Sensitive
			p.cfg.SetVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			p.cfg.SetVariableMap[variable.Name].Type = variable.Type
			if err := p.checkVariablePattern(variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		p.setVariableInConfig(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := interactive.PromptVariable(variable)

			if err != nil {
				return err
			}

			p.setVariableInConfig(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := p.checkVariablePattern(variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}

func (p *Packager) setVariableInConfig(name, value string, sensitive bool, autoIndent bool, varType types.VariableType) {
	p.cfg.SetVariableMap[name] = &types.ZarfSetVariable{
		Name:       name,
		Value:      value,
		Sensitive:  sensitive,
		AutoIndent: autoIndent,
		Type:       varType,
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

// findComponentTemplatesAndReload appends ###ZARF_COMPONENT_NAME###  for each component, assigns value, and reloads
func (p *Packager) findComponentTemplatesAndReload() error {
	// iterate through components to and find all ###ZARF_COMPONENT_NAME, assign to component Name and value
	for i, component := range p.cfg.Pkg.Components {
		mappings := map[string]string{}
		mappings["###ZARF_COMPONENT_NAME###"] = component.Name
		err := utils.ReloadYamlTemplate(&p.cfg.Pkg.Components[i], mappings)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkVariablePattern checks to see if a current variable is set to a value that matches its pattern
func (p *Packager) checkVariablePattern(name, pattern string) error {
	if regexp.MustCompile(pattern).MatchString(p.cfg.SetVariableMap[name].Value) {
		return nil
	}

	return fmt.Errorf("provided value for variable %q does not match pattern \"%s\"", name, pattern)
}
