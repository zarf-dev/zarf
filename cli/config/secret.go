package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/defenseunicorns/zarf/cli/internal/message"
)

type SecretSelector = string

type SecretMap struct {
	length   int
	computed string
	valid    bool
}

const (
	StateRegistryPush   SecretSelector = "registry-push"
	StateRegistryPull   SecretSelector = "registry-pull"
	StateRegistrySecret SecretSelector = "registry-secret"
	StateGitPush        SecretSelector = "git-push"
	StateGitPull        SecretSelector = "git-pull"
	StateLogging        SecretSelector = "logging"
)

var selectors = map[SecretSelector]SecretMap{
	StateRegistryPush:   {length: 48},
	StateRegistryPull:   {length: 48},
	StateRegistrySecret: {length: 48},
	StateGitPush:        {length: 24},
	StateGitPull:        {length: 24},
	StateLogging:        {length: 24},
}

func GetSecret(selector SecretSelector) string {
	message.Debugf("config.GetSecret(%v)", selector)
	if match, ok := selectors[selector]; ok {
		return match.computed
	}
	return ""
}

func initSecrets() {
	message.Debug("config.initSecrets()")
	for filter, selector := range selectors {
		output, err := loadSecret(filter, selector.length)
		if err != nil {
			message.Debug(err)
		} else {
			selector.valid = true
			selector.computed = output
			selectors[filter] = selector
		}
	}
}

func loadSecret(filter SecretSelector, length int) (string, error) {
	message.Debugf("config.loadSecret(%v, %v)", filter, length)
	if state.Secret == "" {
		return "", fmt.Errorf("invalid root secret in the ZarfState")
	}
	hash := sha256.New()
	text := fmt.Sprintf("%s:%s", filter, state.Secret)
	hash.Write([]byte(text))
	output := hex.EncodeToString(hash.Sum(nil))[:length]

	if output != "" {
		return output, nil
	} else {
		return "", fmt.Errorf("unable to generate secret for %s", filter)
	}
}
