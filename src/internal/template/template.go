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
	pushUser, errPush := utils.GetHtpasswdString(config.GetContainerRegistryInfo().RegistryPushUser, config.GetContainerRegistryInfo().RegistryPushPassword)
	pullUser, errPull := utils.GetHtpasswdString(config.GetContainerRegistryInfo().RegistryPullUser, config.GetContainerRegistryInfo().RegistryPullPassword)
	if errPush != nil || errPull != nil {
		message.Debug(errPush, errPull)
		message.Fatal(nil, "Unable to define `htpasswd` string for the Zarf user")
	}
	generated.secret.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)

	generated.seedRegistry = config.GetSeedRegistry()
	generated.registry = config.GetRegistry()

	generated.secret.registryPush = config.GetContainerRegistryInfo().RegistryPushPassword
	generated.secret.registryPull = config.GetContainerRegistryInfo().RegistryPullPassword
	generated.secret.registrySecret = config.GetContainerRegistryInfo().RegistrySecret

	generated.secret.gitPush = config.GetState().GitServer.PushPassword
	generated.secret.gitPull = config.GetState().GitServer.ReadPassword

	generated.secret.logging = config.GetSecret(config.StateLogging)

	generated.agentTLS = state.AgentTLS

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
	message.Debugf("template.Apply(%#v, %s)", component, path)

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

	// Include the data injection marker template if the component has data injections
	if len(component.DataInjections) > 0 {
		builtinMap["DATA_INJECTON_MARKER"] = config.GetDataInjectionMarker()
	}

	// Don't template component-specifric variables for every component
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
	templateMap := map[string]string{}
	for key, value := range builtinMap {
		// Builtin keys are always uppercase in the format ###ZARF_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_%s###", key))] = value
	}

	for key, value := range config.SetVariableMap {
		// Variable keys are always uppercase in the format ###ZARF_VAR_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_VAR_%s###", key))] = value
	}

	for _, constant := range config.GetActiveConfig().Constants {
		// Constant keys are always uppercase in the format ###ZARF_CONST_KEY###
		templateMap[strings.ToUpper(fmt.Sprintf("###ZARF_CONST_%s###", constant.Name))] = constant.Value
	}

	message.Debugf("templateMap = %#v", templateMap)
	utils.ReplaceTextTemplate(path, templateMap)
}
