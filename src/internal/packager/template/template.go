// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// TextTemplate represents a value to be templated into a text file.
type TextTemplate struct {
	Sensitive  bool
	AutoIndent bool
	Type       types.VariableType
	Value      string
}

// Values contains the values to be used in the template.
type Values struct {
	config   *types.PackagerConfig
	htpasswd string
}

// Generate returns a Values struct with the values to be used in the template.
func Generate(cfg *types.PackagerConfig) (*Values, error) {
	message.Debug("template.Generate()")
	var generated Values

	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	generated.config = cfg

	if cfg.State == nil {
		return &generated, nil
	}

	regInfo := cfg.State.RegistryInfo

	// Only calculate this for internal registries to allow longer external passwords
	if regInfo.InternalRegistry {
		pushUser, err := utils.GetHtpasswdString(regInfo.PushUsername, regInfo.PushPassword)
		if err != nil {
			return nil, fmt.Errorf("error generating htpasswd string: %w", err)
		}

		pullUser, err := utils.GetHtpasswdString(regInfo.PullUsername, regInfo.PullPassword)
		if err != nil {
			return nil, fmt.Errorf("error generating htpasswd string: %w", err)
		}

		generated.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)
	}

	return &generated, nil
}

// Ready returns true if the Values struct is ready to be used in the template.
func (values *Values) Ready() bool {
	return values.config.State != nil
}

// SetState sets the state
func (values *Values) SetState(state *types.ZarfState) {
	values.config.State = state
}

// GetVariables returns the variables to be used in the template.
func (values *Values) GetVariables(component types.ZarfComponent) (templateMap map[string]*TextTemplate, deprecations map[string]string) {
	templateMap = make(map[string]*TextTemplate)

	depMarkerOld := "DATA_INJECTON_MARKER"
	depMarkerNew := "DATA_INJECTION_MARKER"
	deprecations = map[string]string{
		fmt.Sprintf("###ZARF_%s###", depMarkerOld): fmt.Sprintf("###ZARF_%s###", depMarkerNew),
	}

	if values.config.State != nil {
		regInfo := values.config.State.RegistryInfo
		gitInfo := values.config.State.GitServer

		builtinMap := map[string]string{
			"STORAGE_CLASS": values.config.State.StorageClass,

			// Registry info
			"REGISTRY":           regInfo.Address,
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
			agentTLS := values.config.State.AgentTLS
			builtinMap["AGENT_CRT"] = base64.StdEncoding.EncodeToString(agentTLS.Cert)
			builtinMap["AGENT_KEY"] = base64.StdEncoding.EncodeToString(agentTLS.Key)
			builtinMap["AGENT_CA"] = base64.StdEncoding.EncodeToString(agentTLS.CA)

		case "zarf-seed-registry", "zarf-registry":
			builtinMap["SEED_REGISTRY"] = fmt.Sprintf("%s:%s", helpers.IPV4Localhost, config.ZarfSeedPort)
			builtinMap["HTPASSWD"] = values.htpasswd
			builtinMap["REGISTRY_SECRET"] = regInfo.Secret

		case "logging":
			builtinMap["LOGGING_AUTH"] = values.config.State.LoggingSecret
		}

		// Iterate over any custom variables and add them to the mappings for templating
		for key, value := range builtinMap {
			// Builtin keys are always uppercase in the format ###ZARF_KEY###
			templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = &TextTemplate{
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

	for key, variable := range values.config.SetVariableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_VAR_%s###", key))] = &TextTemplate{
			Value:      variable.Value,
			Sensitive:  variable.Sensitive,
			AutoIndent: variable.AutoIndent,
			Type:       variable.Type,
		}
	}

	for _, constant := range values.config.Pkg.Constants {
		// Constant keys are always uppercase in the format ###ZARF_CONST_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_CONST_%s###", constant.Name))] = &TextTemplate{
			Value:      constant.Value,
			AutoIndent: constant.AutoIndent,
		}
	}

	debugPrintTemplateMap(templateMap)
	message.Debugf("deprecations = %#v", deprecations)

	return templateMap, deprecations
}

// Apply renders the template and writes the result to the given path.
func (values *Values) Apply(component types.ZarfComponent, path string, ignoreReady bool) error {
	// If Apply() is called before all values are loaded, fail unless ignoreReady is true
	if !values.Ready() && !ignoreReady {
		return fmt.Errorf("template.Apply() called before template.Generate()")
	}

	templateMap, deprecations := values.GetVariables(component)
	err := ReplaceTextTemplate(path, templateMap, deprecations, "###ZARF_[A-Z0-9_]+###")

	return err
}

// ReplaceTextTemplate loads a file from a given path, replaces text in it and writes it back in place.
func ReplaceTextTemplate(path string, mappings map[string]*TextTemplate, deprecations map[string]string, templateRegex string) error {
	textFile, err := os.Open(path)
	if err != nil {
		return err
	}

	// This regex takes a line and parses the text before and after a discovered template: https://regex101.com/r/ilUxAz/1
	regexTemplateLine := regexp.MustCompile(fmt.Sprintf("(?P<preTemplate>.*?)(?P<template>%s)(?P<postTemplate>.*)", templateRegex))

	fileScanner := bufio.NewScanner(textFile)

	// Set the buffer to 1 MiB to handle long lines (i.e. base64 text in a secret)
	// 1 MiB is around the documented maximum size for secrets and configmaps
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	fileScanner.Buffer(buf, maxCapacity)

	// Set the scanner to split on new lines
	fileScanner.Split(bufio.ScanLines)

	text := ""

	for fileScanner.Scan() {
		line := fileScanner.Text()

		for {
			matches := regexTemplateLine.FindStringSubmatch(line)

			// No template left on this line so move on
			if len(matches) == 0 {
				text += fmt.Sprintln(line)
				break
			}

			preTemplate := matches[regexTemplateLine.SubexpIndex("preTemplate")]
			templateKey := matches[regexTemplateLine.SubexpIndex("template")]

			_, present := deprecations[templateKey]
			if present {
				message.Warnf("This Zarf Package uses a deprecated variable: '%s' changed to '%s'.  Please notify your package creator for an update.", templateKey, deprecations[templateKey])
			}

			template := mappings[templateKey]

			// Check if the template is nil (present), use the original templateKey if not (so that it is not replaced).
			value := templateKey
			if template != nil {
				value = template.Value

				// Check if the value is a file type and load the value contents from the file
				if template.Type == types.FileVariableType && value != "" {
					if isText, err := helpers.IsTextFile(value); err != nil || !isText {
						message.Warnf("Refusing to load a non-text file for templating %s", templateKey)
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					contents, err := os.ReadFile(value)
					if err != nil {
						message.Warnf("Unable to read file for templating - skipping: %s", err.Error())
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					value = string(contents)
				}

				// Check if the value is autoIndented and add the correct spacing
				if template.AutoIndent {
					indent := fmt.Sprintf("\n%s", strings.Repeat(" ", len(preTemplate)))
					value = strings.ReplaceAll(value, "\n", indent)
				}
			}

			// Add the processed text and continue processing the line
			text += fmt.Sprintf("%s%s", preTemplate, value)
			line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
		}
	}

	textFile.Close()

	return os.WriteFile(path, []byte(text), helpers.ReadWriteUser)

}

func debugPrintTemplateMap(templateMap map[string]*TextTemplate) {
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
