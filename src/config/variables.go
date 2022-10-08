package config

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
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
	for key, value := range DeployOptions.SetVariables {
		SetVariableMap[key] = value
	}

	for _, variable := range active.Variables {
		_, present := SetVariableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			continue
		}

		// Check if the user did not `--set` a non-defaulted variable and is using `--confirm`
		if CommonOptions.Confirm && variable.Default == nil {
			return fmt.Errorf("variable '%s' has no 'default'; when using '--confirm' you must specify '%s' with '--set' or a config file",
				variable.Name, variable.Name)
		}

		// Initially set the variable to the default (if it exists) (may be overridden by a prompt)
		if variable.Default != nil {
			SetVariableMap[variable.Name] = *variable.Default
		}

		// Prompt the user for a value if in interactive mode
		if !variable.NoPrompt && !CommonOptions.Confirm {
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
	}

	if variable.Default != nil {
		prompt.Default = *variable.Default
	}

	if err = survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
