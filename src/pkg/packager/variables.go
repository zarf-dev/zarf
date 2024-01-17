// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/types"
)

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

// checkVariablePattern checks to see if a current variable is set to a value that matches its pattern
func (p *Packager) checkVariablePattern(name, pattern string) error {
	if regexp.MustCompile(pattern).MatchString(p.cfg.SetVariableMap[name].Value) {
		return nil
	}

	return fmt.Errorf("provided value for variable %q does not match pattern \"%s\"", name, pattern)
}
