// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"encoding/base64"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

type DockerConfig struct {
	Auths DockerConfigEntry `json:"auths"`
}

type DockerConfigEntry map[string]DockerConfigEntryWithAuth

type DockerConfigEntryWithAuth struct {
	Auth string `json:"auth"`
}

func (c *Cluster) GenerateRegistryPullCreds(namespace, name string) *corev1.Secret {
	message.Debugf("k8s.GenerateRegistryPullCreds(%s, %s)", namespace, name)

	secretDockerConfig := c.Kube.GenerateSecret(namespace, name, corev1.SecretTypeDockerConfigJson)

	// Get the registry credentials from the ZarfState secret
	zarfState, err := c.LoadZarfState()
	if err != nil {
		message.Fatalf(err, "Unable to load the Zarf state to get the registry credentials")
	}
	credential := zarfState.RegistryInfo.PullPassword
	if credential == "" {
		message.Fatalf(nil, "Generate pull cred failed")
	}

	// Auth field must be username:password and base64 encoded
	fieldValue := zarfState.RegistryInfo.PullUsername + ":" + credential
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	registry := config.GetRegistry(zarfState)
	// Create the expected structure for the dockerconfigjson
	dockerConfigJSON := DockerConfig{
		Auths: DockerConfigEntry{
			registry: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}

	// Convert to JSON
	dockerConfigData, err := json.Marshal(dockerConfigJSON)
	if err != nil {
		message.Fatalf(err, "Unable to create the embedded registry secret")
	}

	// Add to the secret data
	secretDockerConfig.Data[".dockerconfigjson"] = dockerConfigData

	return secretDockerConfig
}
