// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/interactive"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

const (
	depMarker = "DATA_INJECTION_MARKER"
)

// GetZarfVariableConfig gets a variable configuration specific to Zarf
func GetZarfVariableConfig(ctx context.Context) *variables.VariableConfig {
	prompt := func(variable v1alpha1.InteractiveVariable) (value string, err error) {
		if config.CommonOptions.Confirm {
			return variable.Default, nil
		}
		return interactive.PromptVariable(ctx, variable)
	}

	if logger.Enabled(ctx) {
		return variables.New("zarf", prompt, logger.From(ctx))
	}
	return variables.New("zarf", prompt, slog.New(message.ZarfHandler{}))
}

// GetZarfTemplates returns the template keys and values to be used for templating.
func GetZarfTemplates(ctx context.Context, componentName string, state *types.ZarfState) (templateMap map[string]*variables.TextTemplate, err error) {
	templateMap = make(map[string]*variables.TextTemplate)

	if state != nil {
		regInfo := state.RegistryInfo
		gitInfo := state.GitServer

		builtinMap := map[string]string{
			"STORAGE_CLASS": state.StorageClass,

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

		builtinMap[depMarker] = config.GetDataInjectionMarker()

		// Don't template component-specific variables for every component
		switch componentName {
		case "zarf-agent":
			agentTLS := state.AgentTLS
			builtinMap["AGENT_CRT"] = base64.StdEncoding.EncodeToString(agentTLS.Cert)
			builtinMap["AGENT_KEY"] = base64.StdEncoding.EncodeToString(agentTLS.Key)
			builtinMap["AGENT_CA"] = base64.StdEncoding.EncodeToString(agentTLS.CA)

		case "zarf-seed-registry", "zarf-registry":
			builtinMap["SEED_REGISTRY"] = fmt.Sprintf("%s:%s", helpers.IPV4Localhost, config.ZarfSeedPort)
			htpasswd, err := generateHtpasswd(&regInfo)
			if err != nil {
				return templateMap, err
			}
			builtinMap["HTPASSWD"] = htpasswd
			builtinMap["REGISTRY_SECRET"] = regInfo.Secret
		}

		// Iterate over any custom variables and add them to the mappings for templating
		for key, value := range builtinMap {
			// Builtin keys are always uppercase in the format ###ZARF_KEY###
			templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = &variables.TextTemplate{
				Value: value,
			}

			if key == "REGISTRY_SECRET" || key == "HTPASSWD" ||
				key == "AGENT_CA" || key == "AGENT_KEY" || key == "AGENT_CRT" || key == "GIT_AUTH_PULL" ||
				key == "GIT_AUTH_PUSH" || key == "REGISTRY_AUTH_PULL" || key == "REGISTRY_AUTH_PUSH" {
				// Sanitize any builtin templates that are sensitive
				templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))].Sensitive = true
			}
		}
	}

	err = debugPrintTemplateMap(ctx, templateMap)
	if err != nil {
		return nil, err
	}

	return templateMap, nil
}

// generateHtpasswd returns an htpasswd string for the current state's RegistryInfo.
func generateHtpasswd(regInfo *types.RegistryInfo) (string, error) {
	// Only calculate this for internal registries to allow longer external passwords
	if regInfo.IsInternal() {
		pushUser, err := utils.GetHtpasswdString(regInfo.PushUsername, regInfo.PushPassword)
		if err != nil {
			return "", fmt.Errorf("error generating htpasswd string: %w", err)
		}

		pullUser, err := utils.GetHtpasswdString(regInfo.PullUsername, regInfo.PullPassword)
		if err != nil {
			return "", fmt.Errorf("error generating htpasswd string: %w", err)
		}

		return fmt.Sprintf("%s\\n%s", pushUser, pullUser), nil
	}

	return "", nil
}

func debugPrintTemplateMap(ctx context.Context, templateMap map[string]*variables.TextTemplate) error {
	sanitizedMap := getSanitizedTemplateMap(templateMap)

	b, err := json.MarshalIndent(sanitizedMap, "", "  ")
	if err != nil {
		return err
	}

	message.Debug(fmt.Sprintf("templateMap = %s", string(b)))
	logger.From(ctx).Debug("cluster.debugPrintTemplateMap", "templateMap", sanitizedMap)
	return nil
}

func getSanitizedTemplateMap(templateMap map[string]*variables.TextTemplate) map[string]string {
	sanitizedMap := make(map[string]string, len(templateMap))
	for key, template := range templateMap {
		if template == nil {
			sanitizedMap[key] = ""
		} else if template.Sensitive {
			sanitizedMap[key] = "**sanitized**"
		} else {
			sanitizedMap[key] = template.Value
		}
	}
	return sanitizedMap
}
