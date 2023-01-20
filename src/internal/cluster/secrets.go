// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// DockerConfig contains the authentication information from the machine's docker config.
type DockerConfig struct {
	Auths DockerConfigEntry `json:"auths"`
}

// DockerConfigEntry contains a map of DockerConfigEntryWithAuth for a registry.
type DockerConfigEntry map[string]DockerConfigEntryWithAuth

// DockerConfigEntryWithAuth contains a docker config authentication string.
type DockerConfigEntryWithAuth struct {
	Auth string `json:"auth"`
}

// GenerateRegistryPullCreds generates a secret containing the registry credentials.
func (c *Cluster) GenerateRegistryPullCreds(namespace, name string) (*corev1.Secret, error) {
	message.Debugf("k8s.GenerateRegistryPullCreds(%s, %s)", namespace, name)

	secretDockerConfig := c.Kube.GenerateSecret(namespace, name, corev1.SecretTypeDockerConfigJson)

	// Get the registry credentials from the ZarfState secret
	zarfState, err := c.LoadZarfState()
	if err != nil {
		return nil, err
	}
	credential := zarfState.RegistryInfo.PullPassword
	if credential == "" {
		return nil, fmt.Errorf("generating pull credential failed")
	}

	// Auth field must be username:password and base64 encoded
	fieldValue := zarfState.RegistryInfo.PullUsername + ":" + credential
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	registry := zarfState.RegistryInfo.Address
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
		return nil, err
	}

	// Add to the secret data
	secretDockerConfig.Data[".dockerconfigjson"] = dockerConfigData

	return secretDockerConfig, nil
}
