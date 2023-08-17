// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables provides functions for templating yaml files based on Zarf variables and constants.
package variables

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Values contains the values to be used in the template.
type Values struct {
	state       *types.ZarfState
	variableMap map[string]*types.ZarfSetVariable
	constants   []types.ZarfPackageConstant
	htpasswd    string
}

func New(pkg types.ZarfPackage, setVariables map[string]string) (*Values, error) {
	values := &Values{
		variableMap: map[string]*types.ZarfSetVariable{},
		constants:   pkg.Constants,
	}

	if err := values.setVariableMap(pkg.Variables, setVariables); err != nil {
		return nil, err
	}

	return values, nil
}

// SetState returns a Values struct with the values to be used in the template.
func (values *Values) SetState(state *types.ZarfState) error {
	if state == nil {
		return nil
	}

	values.state = state

	regInfo := state.RegistryInfo

	// Only calculate this for internal registries to allow longer external passwords
	if regInfo.InternalRegistry {
		pushUser, err := utils.GetHtpasswdString(regInfo.PushUsername, regInfo.PushPassword)
		if err != nil {
			return fmt.Errorf("error generating htpasswd string: %w", err)
		}

		pullUser, err := utils.GetHtpasswdString(regInfo.PullUsername, regInfo.PullPassword)
		if err != nil {
			return fmt.Errorf("error generating htpasswd string: %w", err)
		}

		values.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)
	}

	return nil
}

// HasState returns true if the Values struct has its state and is ready to be used in the template.
func (values *Values) HasState() bool {
	return values.state != nil
}

// GetVariables returns the variables to be used in the template.
func (values *Values) GetVariables(component types.ZarfComponent) (templateMap map[string]*utils.TextTemplate, deprecations map[string]string) {
	templateMap = make(map[string]*utils.TextTemplate)

	depMarkerOld := "DATA_INJECTON_MARKER"
	depMarkerNew := "DATA_INJECTION_MARKER"
	deprecations = map[string]string{
		fmt.Sprintf("###ZARF_%s###", depMarkerOld): fmt.Sprintf("###ZARF_%s###", depMarkerNew),
	}

	if values.state != nil {
		regInfo := values.state.RegistryInfo
		gitInfo := values.state.GitServer

		builtinMap := map[string]string{
			"STORAGE_CLASS": values.state.StorageClass,

			// Registry info
			"REGISTRY":           values.state.RegistryInfo.Address,
			"NODEPORT":           fmt.Sprintf("%d", regInfo.NodePort),
			"REGISTRY_AUTH_PUSH": regInfo.PushPassword,
			"REGISTRY_AUTH_PULL": regInfo.PullPassword,

			// Git server info
			"GIT_PUSH":      gitInfo.PushUsername,
			"GIT_AUTH_PUSH": gitInfo.PushPassword,
			"GIT_PULL":      gitInfo.PullUsername,
			"GIT_AUTH_PULL": gitInfo.PullPassword,
		}

		// Include the data injection marker template if the component has data injections
		if len(component.DataInjections) > 0 {
			// Preserve existing misspelling for backwards compatibility
			builtinMap[depMarkerOld] = config.GetDataInjectionMarker()
			builtinMap[depMarkerNew] = config.GetDataInjectionMarker()
		}

		// Don't template component-specific variables for every component
		switch component.Name {
		case "zarf-agent":
			agentTLS := values.state.AgentTLS
			builtinMap["AGENT_CRT"] = base64.StdEncoding.EncodeToString(agentTLS.Cert)
			builtinMap["AGENT_KEY"] = base64.StdEncoding.EncodeToString(agentTLS.Key)
			builtinMap["AGENT_CA"] = base64.StdEncoding.EncodeToString(agentTLS.CA)

		case "zarf-seed-registry", "zarf-registry":
			builtinMap["SEED_REGISTRY"] = fmt.Sprintf("%s:%s", config.IPV4Localhost, config.ZarfSeedPort)
			builtinMap["HTPASSWD"] = values.htpasswd
			builtinMap["REGISTRY_SECRET"] = regInfo.Secret

		case "logging":
			builtinMap["LOGGING_AUTH"] = values.state.LoggingSecret
		}

		// Iterate over any custom variables and add them to the mappings for templating
		for key, value := range builtinMap {
			// Builtin keys are always uppercase in the format ###ZARF_KEY###
			templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = &utils.TextTemplate{
				Value: value,
			}

			if key == "LOGGING_AUTH" || key == "REGISTRY_SECRET" || key == "HTPASSWD" ||
				key == "AGENT_CA" || key == "AGENT_KEY" || key == "AGENT_CRT" || key == "GIT_AUTH_PULL" ||
				key == "GIT_AUTH_PUSH" || key == "REGISTRY_AUTH_PULL" || key == "REGISTRY_AUTH_PUSH" {
				// Sanitize any builtin templates that are sensitive
				templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))].Sensitive = true
			}
		}
	}

	for key, variable := range values.variableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_VAR_%s###", key))] = &utils.TextTemplate{
			Value:      variable.Value,
			Sensitive:  variable.Sensitive,
			AutoIndent: variable.AutoIndent,
			Type:       variable.Type,
		}
	}

	for _, constant := range values.constants {
		// Constant keys are always uppercase in the format ###ZARF_CONST_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_CONST_%s###", constant.Name))] = &utils.TextTemplate{
			Value:      constant.Value,
			AutoIndent: constant.AutoIndent,
		}
	}

	debugPrintTemplateMap(templateMap)
	message.Debugf("deprecations = %#v", deprecations)

	return templateMap, deprecations
}

// Apply renders the template to the given file and writes the result to the given path.
func (values *Values) Apply(component types.ZarfComponent, path string) error {
	templateMap, deprecations := values.GetVariables(component)
	err := utils.ReplaceTextTemplate(path, templateMap, deprecations, "###ZARF_[A-Z0-9_]+###")

	return err
}

// SetVariable sets an individual variable
func (values *Values) SetVariable(name, value string, sensitive bool, autoIndent bool, varType types.VariableType) {
	values.variableMap[name] = &types.ZarfSetVariable{
		Name:       name,
		Value:      value,
		Sensitive:  sensitive,
		AutoIndent: autoIndent,
		Type:       varType,
	}
}

// setVariableMap handles setting the active variables used to template component files.
func (values *Values) setVariableMap(variables []types.ZarfPackageVariable, setVariables map[string]string) error {
	for name, value := range setVariables {
		values.SetVariable(name, value, false, false, "")
	}

	for _, variable := range variables {
		_, present := values.variableMap[variable.Name]

		// Variable is present, no need to continue checking
		if present {
			values.variableMap[variable.Name].Sensitive = variable.Sensitive
			values.variableMap[variable.Name].AutoIndent = variable.AutoIndent
			values.variableMap[variable.Name].Type = variable.Type
			continue
		}

		// First set default (may be overridden by prompt)
		values.SetVariable(variable.Name, variable.Default, variable.Sensitive, variable.AutoIndent, variable.Type)

		// Variable is set to prompt the user
		if variable.Prompt && !config.CommonOptions.Confirm {
			// Prompt the user for the variable
			val, err := interactive.PromptVariable(variable)

			if err != nil {
				return err
			}

			values.SetVariable(variable.Name, val, variable.Sensitive, variable.AutoIndent, variable.Type)
		}
	}

	return nil
}

func debugPrintTemplateMap(templateMap map[string]*utils.TextTemplate) {
	debugText := "templateMap = { "

	for key, template := range templateMap {
		if template.Sensitive {
			debugText += fmt.Sprintf("\"%s\": \"**sanitized**\", ", key)
		} else {
			debugText += fmt.Sprintf("\"%s\": \"%s\", ", key, template.Value)
		}
	}

	debugText += " }"

	message.Debug(debugText)
}
