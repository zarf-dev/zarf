// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for working with variables within Zarf packages.
package variables

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/types"
)

// SetVariableMapInConfig handles setting the active variables used to template component files.
func SetVariableMapInConfig(cfg *types.PackagerConfig) error {
	for name, value := range cfg.PkgOpts.SetVariables {
		cfg.SetVariable(name, value, false, false, "")
	}

	for _, variable := range cfg.Pkg.Variables {
		_, present := cfg.SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			cfg.SetVariableMap[variable.Name].Sensitive = variable.Sensitive
			cfg.SetVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			cfg.SetVariableMap[variable.Name].Type = variable.Type
			if err := cfg.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		cfg.SetVariable(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := interactive.PromptVariable(variable)

			if err != nil {
				return err
			}

			cfg.SetVariable(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := cfg.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}
