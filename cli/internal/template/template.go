package template

import (
	"fmt"
	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

type Values struct {
	state          types.ZarfState
	htpasswd       string
	seedRegistry   string
	registry       string
	registryPush   string
	registryPull   string
	registrySecret string
	gitPush        string
	gitPull        string
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
	generated.htpasswd = fmt.Sprintf("%s\\n%s", pushUser, pullUser)

	generated.registry = config.GetRegistry()
	generated.seedRegistry = config.GetSeedRegistry()

	generated.registryPush = config.GetSecret(config.StateRegistryPush)
	generated.registryPull = config.GetSecret(config.StateRegistryPull)
	generated.registrySecret = config.GetSecret(config.StateRegistrySecret)

	generated.gitPush = config.GetSecret(config.StateGitPush)
	generated.gitPull = config.GetSecret(config.StateGitPull)

	message.Debugf("Template values: %v", generated)
	return generated
}

func (values Values) Ready() bool {
	return values.htpasswd != ""
}

func (values Values) GetRegistry() string {
	message.Debug("template.GetRegistry()")
	return values.registry
}

func (values Values) Apply(path string) {
	message.Debugf("template.Apply(%s)", path)

	if !values.Ready() {
		// This should only occur if the state couldn't be pulled or on init if a template is attempted before the pre-seed stage
		message.Fatalf(nil, "template.Apply() called bofore template.Generate()")
	}

	mappings := map[string]string{
		"STORAGE_CLASS":      values.state.StorageClass,
		"SEED_REGISTRY":      values.seedRegistry,
		"REGISTRY":           values.registry,
		"REGISTRY_NODEPORT":  values.state.Registry.NodePort,
		"REGISTRY_SECRET":    values.registrySecret,
		"REGISTRY_AUTH_PUSH": values.registryPush,
		"REGISTRY_AUTH_PULL": values.registryPull,
		"GIT_AUTH_PUSH":      values.gitPush,
		"GIT_AUTH_PULL":      values.gitPull,
		"HTPASSWD":           values.htpasswd,
	}

	message.Debug(mappings)

	for template, value := range mappings {
		template = fmt.Sprintf("###ZARF_%s###", template)
		utils.ReplaceText(path, template, value)
	}
}
