// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	fieldValue := registryInfo.PullUsername + ":" + registryInfo.PullPassword
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))
	registry := registryInfo.Address
	dockerConfigJSON := DockerConfig{
		Auths: DockerConfigEntry{
			registry: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}
	dockerConfigData, err := json.Marshal(dockerConfigJSON)
	// TODO: Return an error here
	if err != nil {
		message.WarnErrf(err, "Unable to marshal the .dockerconfigjson secret data for the image pull secret")
	}

	secretDockerConfig := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			".dockerconfigjson": dockerConfigData,
		},
	}
	return secretDockerConfig
}

// GenerateGitPullCreds generates a secret containing the git credentials.
func (c *Cluster) GenerateGitPullCreds(namespace, name string, gitServerInfo types.GitServerInfo) *corev1.Secret {
	message.Debugf("k8s.GenerateGitPullCreds(%s, %s, gitServerInfo)", namespace, name)
	gitServerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{},
		StringData: map[string]string{
			"username": gitServerInfo.PullUsername,
			"password": gitServerInfo.PullPassword,
		},
	}
	return gitServerSecret
}

// UpdateZarfManagedImageSecrets updates all Zarf-managed image secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedImageSecrets(ctx context.Context, state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed image secrets")
	defer spinner.Stop()

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
		return
	}

	// Update all image pull secrets
	for _, namespace := range namespaceList.Items {
		currentRegistrySecret, err := c.Clientset.CoreV1().Secrets(namespace.Name).Get(ctx, config.ZarfImagePullSecretName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		// Check if this is a Zarf managed secret or is in a namespace the Zarf agent will take action in
		if currentRegistrySecret.Labels[config.ZarfManagedByLabel] == "zarf" ||
			(namespace.Labels[agentLabel] != "skip" && namespace.Labels[agentLabel] != "ignore") {
			spinner.Updatef("Updating existing Zarf-managed image secret for namespace: '%s'", namespace.Name)

			// Create the secret
			newRegistrySecret := c.GenerateRegistryPullCreds(namespace.Name, config.ZarfImagePullSecretName, state.RegistryInfo)
			// TODO: Avoid using reflections, there is probably a better way to do this
			if !reflect.DeepEqual(currentRegistrySecret.Data, newRegistrySecret.Data) {
				// Create or update the zarf registry secret
				// TODO: Refactor this
				// TODO: An error should actually be returned here, not a warning
				_, err = c.Clientset.CoreV1().Secrets(newRegistrySecret.Namespace).Get(ctx, newRegistrySecret.Name, metav1.GetOptions{})
				if err != nil {
					_, err = c.Clientset.CoreV1().Secrets(newRegistrySecret.Namespace).Create(ctx, newRegistrySecret, metav1.CreateOptions{})
					if err != nil {
						message.WarnErrf(err, "Problem creating registry secret for the %s namespace", namespace.Name)
					}
				} else {
					_, err = c.Clientset.CoreV1().Secrets(newRegistrySecret.Namespace).Update(ctx, newRegistrySecret, metav1.UpdateOptions{})
					if err != nil {
						message.WarnErrf(err, "Problem creating registry secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
	}
	spinner.Success()
}

// UpdateZarfManagedGitSecrets updates all Zarf-managed git secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedGitSecrets(ctx context.Context, state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed git secrets")
	defer spinner.Stop()

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
		return
	}

	// Update all git pull secrets
	for _, namespace := range namespaceList.Items {
		currentGitSecret, err := c.Clientset.CoreV1().Secrets(namespace.Name).Get(ctx, config.ZarfGitServerSecretName, metav1.GetOptions{})
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
				// TODO: Refactor this
				// TODO: An error should actually be returned here, not a warning
				_, err = c.Clientset.CoreV1().Secrets(newGitSecret.Namespace).Get(ctx, newGitSecret.Name, metav1.GetOptions{})
				if err != nil {
					_, err = c.Clientset.CoreV1().Secrets(newGitSecret.Namespace).Create(ctx, newGitSecret, metav1.CreateOptions{})
					if err != nil {
						message.WarnErrf(err, "Problem creating git server secret for the %s namespace", namespace.Name)
					}
				} else {
					_, err = c.Clientset.CoreV1().Secrets(newGitSecret.Namespace).Update(ctx, newGitSecret, metav1.UpdateOptions{})
					if err != nil {
						message.WarnErrf(err, "Problem creating git server secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
	}
	spinner.Success()
}
