// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for working with variables within Zarf packages.
package variables

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/types"
)

// SetVariableMapInConfig handles setting the active variables used to template component files.
func SetVariableMapInConfig(cfg *types.PackagerConfig) error {
	for name, value := range cfg.PkgOpts.SetVariables {
		SetVariableInConfig(cfg, name, value, false, false, "")
	}

	for _, variable := range cfg.Pkg.Variables {
		_, present := cfg.SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			cfg.SetVariableMap[variable.Name].Sensitive = variable.Sensitive
			cfg.SetVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			cfg.SetVariableMap[variable.Name].Type = variable.Type
			if err := CheckVariablePattern(cfg, variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		SetVariableInConfig(cfg, variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := interactive.PromptVariable(variable)

			if err != nil {
				return err
			}

			SetVariableInConfig(cfg, variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := CheckVariablePattern(cfg, variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}

func SetVariableInConfig(cfg *types.PackagerConfig, name, value string, sensitive bool, autoIndent bool, varType types.VariableType) {
	cfg.SetVariableMap[name] = &types.ZarfSetVariable{
		Name:       name,
		Value:      value,
		Sensitive:  sensitive,
		AutoIndent: autoIndent,
		Type:       varType,
	}
}

// CheckVariablePattern checks to see if a current variable is set to a value that matches its pattern
func CheckVariablePattern(cfg *types.PackagerConfig, name, pattern string) error {
	if regexp.MustCompile(pattern).MatchString(cfg.SetVariableMap[name].Value) {
		return nil
	}

	return fmt.Errorf("provided value for variable %q does not match pattern \"%s\"", name, pattern)
}
