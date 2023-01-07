// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Values contains the values to be used in the template.
type Values struct {
	config   *types.PackagerConfig
	registry string
	htpasswd string
}

// Generate returns a Values struct with the values to be used in the template.
func Generate(cfg *types.PackagerConfig) (Values, error) {
	message.Debug("template.Generate()")
	var generated Values

	if cfg == nil {
		return generated, fmt.Errorf("config is nil")
	}

	generated.config = cfg

	regInfo := cfg.State.RegistryInfo

	pushUser, err := utils.GetHtpasswdString(regInfo.PushUsername, regInfo.PushPassword)
	if err != nil {
		return generated, fmt.Errorf("error generating htpasswd string: %w", err)
	}

	pullUser, err := utils.GetHtpasswdString(regInfo.PullUsername, regInfo.PullPassword)
	if err != nil {
		return generated, fmt.Errorf("error generating htpasswd string: %w", err)
	}

	generated.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)

	generated.registry = config.GetRegistry(cfg.State)

	return generated, nil
}

// Ready returns true if the Values struct is ready to be used in the template.
func (values Values) Ready() bool {
	return values.config.State.Distro != ""
}

// GetRegistry returns the registry address.
func (values Values) GetRegistry() string {
	return values.registry
}

// Apply renders the template and writes the result to the given path.
func (values Values) Apply(component types.ZarfComponent, path string) {
	message.Debugf("template.Apply(%#v, %s)", component, path)

	if !values.Ready() {
		// This should only occur if the state couldn't be pulled or on init if a template is attempted before the pre-seed stage
		message.Fatalf(nil, "template.Apply() called before template.Generate()")
	}

	regInfo := values.config.State.RegistryInfo
	gitInfo := values.config.State.GitServer

	builtinMap := map[string]string{
		"STORAGE_CLASS": values.config.State.StorageClass,

		// Registry info
		"REGISTRY":           values.registry,
		"NODEPORT":           fmt.Sprintf("%d", regInfo.NodePort),
		"REGISTRY_AUTH_PUSH": regInfo.PushPassword,
		"REGISTRY_AUTH_PULL": regInfo.PullPassword,

		// Git server info
		"GIT_PUSH":      gitInfo.PushUsername,
		"GIT_AUTH_PUSH": gitInfo.PushPassword,
		"GIT_AUTH_PULL": gitInfo.PullPassword,
	}

	// Include the data injection marker template if the component has data injections
	if len(component.DataInjections) > 0 {
		// Preserve existing misspelling for backwards compatibility
		builtinMap["DATA_INJECTON_MARKER"] = config.GetDataInjectionMarker()
		builtinMap["DATA_INJECTION_MARKER"] = config.GetDataInjectionMarker()
	}

	// Don't template component-specific variables for every component
	switch component.Name {
	case "zarf-agent":
		agentTLS := values.config.State.AgentTLS
		builtinMap["AGENT_CRT"] = base64.StdEncoding.EncodeToString(agentTLS.Cert)
		builtinMap["AGENT_KEY"] = base64.StdEncoding.EncodeToString(agentTLS.Key)
		builtinMap["AGENT_CA"] = base64.StdEncoding.EncodeToString(agentTLS.CA)

	case "zarf-seed-registry", "zarf-registry":
		builtinMap["SEED_REGISTRY"] = fmt.Sprintf("%s:%s", config.IPV4Localhost, config.ZarfSeedPort)
		builtinMap["HTPASSWD"] = values.htpasswd
		builtinMap["REGISTRY_SECRET"] = regInfo.Secret

	case "logging":
		builtinMap["LOGGING_AUTH"] = values.config.State.LoggingSecret
	}

	// Iterate over any custom variables and add them to the mappings for templating
	templateMap := map[string]string{}
	for key, value := range builtinMap {
		// Builtin keys are always uppercase in the format ###ZARF_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = value
	}

	for key, value := range values.config.SetVariableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_VAR_%s###", key))] = value
	}

	for _, constant := range values.config.Pkg.Constants {
		// Constant keys are always uppercase in the format ###ZARF_CONST_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_CONST_%s###", constant.Name))] = constant.Value
	}

	message.Debugf("templateMap = %#v", templateMap)
	utils.ReplaceTextTemplate(path, templateMap)
}
