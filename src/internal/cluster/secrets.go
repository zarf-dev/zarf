// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"encoding/base64"
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
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
func (c *Cluster) GenerateRegistryPullCreds(namespace, name string, registryInfo types.RegistryInfo) *corev1.Secret {
	secretDockerConfig := c.GenerateSecret(namespace, name, corev1.SecretTypeDockerConfigJson)

	// Auth field must be username:password and base64 encoded
	fieldValue := registryInfo.PullUsername + ":" + registryInfo.PullPassword
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	registry := registryInfo.Address
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
		message.WarnErrf(err, "Unable to marshal the .dockerconfigjson secret data for the image pull secret")
	}

	// Add to the secret data
	secretDockerConfig.Data[".dockerconfigjson"] = dockerConfigData

	return secretDockerConfig
}

// GenerateGitPullCreds generates a secret containing the git credentials.
func (c *Cluster) GenerateGitPullCreds(namespace, name string, gitServerInfo types.GitServerInfo) *corev1.Secret {
	message.Debugf("k8s.GenerateGitPullCreds(%s, %s, gitServerInfo)", namespace, name)

	gitServerSecret := c.GenerateSecret(namespace, name, corev1.SecretTypeOpaque)
	gitServerSecret.StringData = map[string]string{
		"username": gitServerInfo.PullUsername,
		"password": gitServerInfo.PullPassword,
	}

	return gitServerSecret
}

// UpdateZarfManagedImageSecrets updates all Zarf-managed image secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedImageSecrets(state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed image secrets")
	defer spinner.Stop()

	if namespaces, err := c.GetNamespaces(); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		// Update all image pull secrets
		for _, namespace := range namespaces.Items {
			currentRegistrySecret, err := c.GetSecret(namespace.Name, config.ZarfImagePullSecretName)
			if err != nil {
				continue
			}

			// Check if this is a Zarf managed secret or is in a namespace the Zarf agent will take action in
			if currentRegistrySecret.Labels[config.ZarfManagedByLabel] == "zarf" ||
				(namespace.Labels[agentLabel] != "skip" && namespace.Labels[agentLabel] != "ignore") {
				spinner.Updatef("Updating existing Zarf-managed image secret for namespace: '%s'", namespace.Name)

				// Create the secret
				newRegistrySecret := c.GenerateRegistryPullCreds(namespace.Name, config.ZarfImagePullSecretName, state.RegistryInfo)
				if !reflect.DeepEqual(currentRegistrySecret.Data, newRegistrySecret.Data) {
					// Create or update the zarf registry secret
					if err := c.CreateOrUpdateSecret(newRegistrySecret); err != nil {
						message.WarnErrf(err, "Problem creating registry secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
		spinner.Success()
	}
}

// UpdateZarfManagedGitSecrets updates all Zarf-managed git secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedGitSecrets(state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed git secrets")
	defer spinner.Stop()

	if namespaces, err := c.GetNamespaces(); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		// Update all git pull secrets
		for _, namespace := range namespaces.Items {
			currentGitSecret, err := c.GetSecret(namespace.Name, config.ZarfGitServerSecretName)
			if err != nil {
				continue
			}

			// Check if this is a Zarf managed secret or is in a namespace the Zarf agent will take action in
			if currentGitSecret.Labels[config.ZarfManagedByLabel] == "zarf" ||
				(namespace.Labels[agentLabel] != "skip" && namespace.Labels[agentLabel] != "ignore") {
				spinner.Updatef("Updating existing Zarf-managed git secret for namespace: '%s'", namespace.Name)

				// Create the secret
				newGitSecret := c.GenerateGitPullCreds(namespace.Name, config.ZarfGitServerSecretName, state.GitServer)
				if !reflect.DeepEqual(currentGitSecret.StringData, newGitSecret.StringData) {
					// Create or update the zarf git secret
					if err := c.CreateOrUpdateSecret(newGitSecret); err != nil {
						message.WarnErrf(err, "Problem creating git server secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
		spinner.Success()
	}
}
