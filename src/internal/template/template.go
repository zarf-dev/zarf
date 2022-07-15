package template

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

type Values struct {
	state        types.ZarfState
	seedRegistry string
	registry     string
	agentTLS     types.GeneratedPKI
	secret       struct {
		htpasswd       string
		registryPush   string
		registryPull   string
		registrySecret string
		gitPush        string
		gitPull        string
		logging        string
	}
}

func Generate() Values {
	message.Debug("template.Generate()")
	var generated Values
	state := config.GetState()

	generated.state = state
	pushUser, errPush := utils.GetHtpasswdString(config.ZarfRegistryPushUser, config.GetSecret(config.StateRegistryPush))
	pullUser, errPull := utils.GetHtpasswdString(config.ZarfRegistryPullUser, config.GetSecret(config.StateRegistryPull))
	if errPush != nil || errPull != nil {
		message.Debug(errPush, errPull)
		message.Fatal(nil, "Unable to define `htpasswd` string for the Zarf user")
	}
	generated.secret.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)

	generated.seedRegistry = config.GetSeedRegistry()
	generated.registry = config.GetRegistry()

	generated.secret.registryPush = config.GetSecret(config.StateRegistryPush)
	generated.secret.registryPull = config.GetSecret(config.StateRegistryPull)
	generated.secret.registrySecret = config.GetSecret(config.StateRegistrySecret)

	generated.secret.gitPush = config.GetSecret(config.StateGitPush)
	generated.secret.gitPull = config.GetSecret(config.StateGitPull)

	generated.secret.logging = config.GetSecret(config.StateLogging)

	generated.agentTLS = state.AgentTLS

	message.Debugf("Template values: %v", generated)
	return generated
}

func (values Values) Ready() bool {
	return values.secret.htpasswd != ""
}

func (values Values) GetRegistry() string {
	message.Debug("template.GetRegistry()")
	return values.registry
}

func (values Values) Apply(component types.ZarfComponent, path string) {
	message.Debugf("template.Apply(%v, %s)", component, path)

	if !values.Ready() {
		// This should only occur if the state couldn't be pulled or on init if a template is attempted before the pre-seed stage
		message.Fatalf(nil, "template.Apply() called before template.Generate()")
	}

	builtinMap := map[string]string{
		"STORAGE_CLASS":      values.state.StorageClass,
		"REGISTRY":           values.registry,
		"NODEPORT":           values.state.NodePort,
		"REGISTRY_AUTH_PUSH": values.secret.registryPush,
		"REGISTRY_AUTH_PULL": values.secret.registryPull,
		"GIT_AUTH_PUSH":      values.secret.gitPush,
		"GIT_AUTH_PULL":      values.secret.gitPull,
	}

	// Don't template component-specific variables for every component
	switch component.Name {
	case "zarf-agent":
		builtinMap["AGENT_CRT"] = base64.StdEncoding.EncodeToString(values.agentTLS.Cert)
		builtinMap["AGENT_KEY"] = base64.StdEncoding.EncodeToString(values.agentTLS.Key)
		builtinMap["AGENT_CA"] = base64.StdEncoding.EncodeToString(values.agentTLS.CA)

	case "zarf-seed-registry", "zarf-registry":
		builtinMap["SEED_REGISTRY"] = values.seedRegistry
		builtinMap["HTPASSWD"] = values.secret.htpasswd
		builtinMap["REGISTRY_SECRET"] = values.secret.registrySecret

	case "logging":
		builtinMap["LOGGING_AUTH"] = values.secret.logging
	}

	// Iterate over any custom variables and add them to the mappings for templating
	// TODO: Handle prompting
	variableMap := config.CommonOptions.SetVariables

	for _, variable := range config.GetActiveConfig().Variables {
		if _, present := variableMap[variable.Name]; !present && variable.Default != nil {
			variableMap[variable.Name] = *variable.Default
		}
	}

	templateMap := map[string]string{}
	for key, value := range builtinMap {
		// Builtin keys are always uppercase in the format ###ZARF_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = value
	}

	for key, value := range variableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_VAR_%s###", key))] = value
	}

	message.Debug(templateMap)

	utils.ReplaceTemplate(path, templateMap)
}
