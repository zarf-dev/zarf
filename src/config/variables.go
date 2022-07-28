package config

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// FillActiveTemplate handles setting the active variables and reloading the base template.
func FillActiveTemplate() error {
	packageVariables, err := utils.FindYamlTemplates(&active, "###ZARF_PKG_VAR_", "###")
	if err != nil {
		return err
	}

	for key := range CommonOptions.SetVariables {
		value := CommonOptions.SetVariables[key]
		packageVariables[key] = &value
	}

	for key, value := range packageVariables {
		if value == nil && !CommonOptions.Confirm {
			setVal, err := promptVariable(key, "")

			if err == nil {
				packageVariables[key] = &setVal
			} else {
				return err
			}
		} else if value == nil {
			return fmt.Errorf("variable '%s' must be '--set' when using the '--confirm' flag", key)
		}
	}

	templateMap := map[string]string{}
	for key, value := range packageVariables {
		// Variable keys are always uppercase in the format ###ZARF_PKG_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_PKG_VAR_%s###", key))] = *value
	}

	return utils.ReloadYamlTemplate(&active, templateMap)
}

// SetActiveVariables handles setting the active variables used to template component files.
func SetActiveVariables() error {
	SetVariableMap = CommonOptions.SetVariables

	for _, variable := range active.Variables {
		_, present := SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			continue
		}

		// First set default (may be overridden by prompt)
		SetVariableMap[variable.Name] = variable.Default

		// Variable is set to prompt the user
		if variable.Prompt {
			// Prompt the user for the variable
			val, err := promptVariable(variable.Name, variable.Default)

			if err != nil {
				return err
			}

			SetVariableMap[variable.Name] = val
		}
	}

	return nil
}

// InjectImportedVariable determines if an imported package variable exists in the active config and adds it if not.
func InjectImportedVariable(importedVariable types.ZarfPackageVariable) {
	presentInActive := false
	for _, configVariable := range active.Variables {
		if configVariable.Name == importedVariable.Name {
			presentInActive = true
		}
	}

	if !presentInActive {
		active.Variables = append(active.Variables, importedVariable)
	}
}

// InjectImportedConstant determines if an imported package constant exists in the active config and adds it if not.
func InjectImportedConstant(importedConstant types.ZarfPackageConstant) {
	presentInActive := false
	for _, configVariable := range active.Constants {
		if configVariable.Name == importedConstant.Name {
			presentInActive = true
		}
	}

	if !presentInActive {
		active.Constants = append(active.Constants, importedConstant)
	}
}

func promptVariable(varName string, varDefault string) (string, error) {
	var value string

	pterm.Println()

	prompt := &survey.Input{
		Message: "Please provide a value for '" + varName + "'",
		Default: varDefault,
	}

	if err := survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
