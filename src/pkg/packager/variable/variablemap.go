package variable

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/types"
)

// The main question is, how do we want to pair up package config and the set variable map
// It definitely is a config, so it makes sense there.
// We also want to split it up from packager.

// SetVariableMapInConfig handles setting the active variables used to template component files.
func SetVariableMapInConfig(cfg types.PackagerConfig) error {
	for name, value := range cfg.PkgOpts.SetVariables {
		cfg.SetVariableMap.SetVariableInConfig(name, value, false, false, "")
	}

	for _, variable := range cfg.Pkg.Variables {
		_, present := cfg.SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			cfg.SetVariableMap[variable.Name].Sensitive = variable.Sensitive
			cfg.SetVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			cfg.SetVariableMap[variable.Name].Type = variable.Type
			if err := cfg.SetVariableMap.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		cfg.SetVariableMap.SetVariableInConfig(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := interactive.PromptVariable(variable)

			if err != nil {
				return err
			}

			cfg.SetVariableMap.SetVariableInConfig(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := cfg.SetVariableMap.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}
