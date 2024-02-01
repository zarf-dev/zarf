// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

import (
	"fmt"
	"regexp"
)

type SetVariableMap map[string]*SetVariable

// setVariableMapInConfig handles setting the active variables used to template component files.
func (sv SetVariableMap) PopulateSetVariableMap(variables []Variable, presetVariables map[string]string, prompt func(variable Variable) (value string, err error)) error {
	for name, value := range presetVariables {
		sv.SetVariableInConfig(name, value, false, false, "")
	}

	for _, variable := range variables {
		_, present := sv[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			sv[variable.Name].Sensitive = variable.Sensitive
			sv[variable.Name].AutoIndent = variable.AutoIndent
			sv[variable.Name].Type = variable.Type
			if err := sv.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		sv.SetVariableInConfig(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt {
			// Prompt the user for the variable
			val, err := prompt(variable)

			if err != nil {
				return err
			}

			sv.SetVariableInConfig(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := sv.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}

func (sv SetVariableMap) SetVariableInConfig(name, value string, sensitive bool, autoIndent bool, varType VariableType) {
	sv[name] = &SetVariable{
		Name:       name,
		Value:      value,
		Sensitive:  sensitive,
		AutoIndent: autoIndent,
		Type:       varType,
	}
}

// checkVariablePattern checks to see if a current variable is set to a value that matches its pattern
func (sv SetVariableMap) CheckVariablePattern(name, pattern string) error {
	if regexp.MustCompile(pattern).MatchString(sv[name].Value) {
		return nil
	}

	return fmt.Errorf("provided value for variable %q does not match pattern \"%s\"", name, pattern)
}
