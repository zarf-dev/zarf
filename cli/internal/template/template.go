package template

import (
	"fmt"
	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

type Values struct {
	state        types.ZarfState
	registry     string
	seedRegistry string
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

	generated.registry = config.GetRegistry()
	generated.seedRegistry = config.GetSeedRegistry()

	generated.secret.registryPush = config.GetSecret(config.StateRegistryPush)
	generated.secret.registryPull = config.GetSecret(config.StateRegistryPull)
	generated.secret.registrySecret = config.GetSecret(config.StateRegistrySecret)

	generated.secret.gitPush = config.GetSecret(config.StateGitPush)
	generated.secret.gitPull = config.GetSecret(config.StateGitPull)

	generated.secret.logging = config.GetSecret(config.StateLogging)

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

func (values Values) Apply(path string) {
	message.Debugf("template.Apply(%s)", path)

	if !values.Ready() {
		// This should only occur if the state couldn't be pulled or on init if a template is attempted before the pre-seed stage
		message.Fatalf(nil, "template.Apply() called before template.Generate()")
	}

	mappings := map[string]string{
		"STORAGE_CLASS":      values.state.StorageClass,
		"SEED_REGISTRY":      values.seedRegistry,
		"REGISTRY":           values.registry,
		"REGISTRY_NODEPORT":  values.state.Registry.NodePort,
		"REGISTRY_SECRET":    values.secret.registrySecret,
		"REGISTRY_AUTH_PUSH": values.secret.registryPush,
		"REGISTRY_AUTH_PULL": values.secret.registryPull,
		"GIT_AUTH_PUSH":      values.secret.gitPush,
		"GIT_AUTH_PULL":      values.secret.gitPull,
		"LOGGING_AUTH":       values.secret.logging,
		"HTPASSWD":           values.secret.htpasswd,
	}

	message.Debug(mappings)

	for template, value := range mappings {
		template = fmt.Sprintf("###ZARF_%s###", template)
		utils.ReplaceText(path, template, value)
	}
}
