package config

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// FillActiveTemplate handles setting the active variables and reloading the base template.
func FillActiveTemplate() error {
	packageVariables, err := utils.FindYamlTemplates(&active, "###ZARF_PKG_VAR_", "###")
	if err != nil {
		return err
	}

	for key := range CreateOptions.SetVariables {
		value := CreateOptions.SetVariables[key]
		// Ensure uppercase for VIPER
		packageVariables[strings.ToUpper(key)] = &value
	}

	for key, value := range packageVariables {
		if value == nil && !CommonOptions.Confirm {
			setVal, err := promptVariable(types.ZarfPackageVariable{
				Name: key,
			})

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
	for key := range DeployOptions.SetVariables {
		value := DeployOptions.SetVariables[key]
		// Ensure uppercase for VIPER
		SetVariableMap[strings.ToUpper(key)] = value
	}

	for _, variable := range active.Variables {
		_, present := SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			continue
		}

		// First set default (may be overridden by prompt)
		SetVariableMap[variable.Name] = variable.Default

		// Variable is set to prompt the user
		if variable.Prompt && !CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := promptVariable(variable)

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

func promptVariable(variable types.ZarfPackageVariable) (value string, err error) {

	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for \"%s\"", variable.Name),
		Default: variable.Default,
	}

	if err = survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
