// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

import (
	"fmt"
	"regexp"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// SetVariableMap represents a map of variable names to their set values
type SetVariableMap map[string]*v1alpha1.SetVariable

// GetSetVariable gets a variable set within a VariableConfig by its name
func (vc *VariableConfig) GetSetVariable(name string) (*v1alpha1.SetVariable, bool) {
	variable, ok := vc.setVariableMap[name]
	return variable, ok
}

// PopulateVariables handles setting the active variables within a VariableConfig's SetVariableMap
func (vc *VariableConfig) PopulateVariables(variables []v1alpha1.InteractiveVariable, presetVariables map[string]string) error {
	for name, value := range presetVariables {
		vc.SetVariable(name, value, false, false, "")
	}

	for _, variable := range variables {
		_, present := vc.setVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			vc.setVariableMap[variable.Name].Sensitive = variable.Sensitive
			vc.setVariableMap[variable.Name].AutoIndent = variable.AutoIndent
			vc.setVariableMap[variable.Name].Type = variable.Type
			if err := vc.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
				return err
			}
			continue
		}

		// First set default (may be overridden by prompt)
		vc.SetVariable(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt {
			// Prompt the user for the variable
			val, err := vc.prompt(variable)

			if err != nil {
				return err
			}

			vc.SetVariable(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}

		if err := vc.CheckVariablePattern(variable.Name, variable.Pattern); err != nil {
			return err
		}
	}

	return nil
}

// SetVariable sets a variable in a VariableConfig's SetVariableMap
func (vc *VariableConfig) SetVariable(name, value string, sensitive bool, autoIndent bool, varType v1alpha1.VariableType) {
	vc.setVariableMap[name] = &v1alpha1.SetVariable{
		Variable: v1alpha1.Variable{
			Name:       name,
			Sensitive:  sensitive,
			AutoIndent: autoIndent,
			Type:       varType,
		},
		Value: value,
	}
}

// CheckVariablePattern checks to see if a current variable is set to a value that matches its pattern
func (vc *VariableConfig) CheckVariablePattern(name, pattern string) error {
	if variable, ok := vc.setVariableMap[name]; ok {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}

		if r.MatchString(variable.Value) {
			return nil
		}

		return fmt.Errorf("provided value for variable %q does not match pattern %q", name, pattern)
	}

	return fmt.Errorf("variable %q was not found in the current variable map", name)
}
