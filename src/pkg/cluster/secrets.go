// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
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
func (c *Cluster) GenerateRegistryPullCreds(ctx context.Context, namespace, name string, registryInfo types.RegistryInfo) *corev1.Secret {
	secretDockerConfig := c.GenerateSecret(namespace, name, corev1.SecretTypeDockerConfigJson)

	// Auth field must be username:password and base64 encoded
	fieldValue := registryInfo.PullUsername + ":" + registryInfo.PullPassword
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	registry := registryInfo.Address

	var dockerConfigJSON DockerConfig

	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	// Build zarf-docker-registry service address string
	svc, _, err := serviceInfoFromNodePortURL(serviceList.Items, registry)
	dockerConfigJSON = DockerConfig{
		Auths: DockerConfigEntry{
			// nodePort for zarf-docker-registry
			registry: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}
	if err == nil {
		kubeDNSRegistryURL := fmt.Sprintf("%s:%s.svc.cluster.local", svc.Namespace, svc.Name)
		dockerConfigJSON.Auths[kubeDNSRegistryURL] = DockerConfigEntryWithAuth{
			Auth: authEncodedValue,
		}
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
// TODO: Refactor to return errors properly.
func (c *Cluster) UpdateZarfManagedImageSecrets(ctx context.Context, state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed image secrets")
	defer spinner.Stop()

	if namespaces, err := c.GetNamespaces(ctx); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		// Update all image pull secrets
		for _, namespace := range namespaces.Items {
			currentRegistrySecret, err := c.GetSecret(ctx, namespace.Name, config.ZarfImagePullSecretName)
			if err != nil {
				continue
			}

			// Check if this is a Zarf managed secret or is in a namespace the Zarf agent will take action in
			if currentRegistrySecret.Labels[k8s.ZarfManagedByLabel] == "zarf" ||
				(namespace.Labels[k8s.AgentLabel] != "skip" && namespace.Labels[k8s.AgentLabel] != "ignore") {
				spinner.Updatef("Updating existing Zarf-managed image secret for namespace: '%s'", namespace.Name)

				// Create the secret
				newRegistrySecret := c.GenerateRegistryPullCreds(ctx, namespace.Name, config.ZarfImagePullSecretName, state.RegistryInfo)
				if !reflect.DeepEqual(currentRegistrySecret.Data, newRegistrySecret.Data) {
					// Create or update the zarf registry secret
					if _, err := c.CreateOrUpdateSecret(ctx, newRegistrySecret); err != nil {
						message.WarnErrf(err, "Problem creating registry secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
		spinner.Success()
	}
}

// UpdateZarfManagedGitSecrets updates all Zarf-managed git secrets in all namespaces based on state
// TODO: Refactor to return errors properly.
func (c *Cluster) UpdateZarfManagedGitSecrets(ctx context.Context, state *types.ZarfState) {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed git secrets")
	defer spinner.Stop()

	if namespaces, err := c.GetNamespaces(ctx); err != nil {
		spinner.Errorf(err, "Unable to get k8s namespaces")
	} else {
		// Update all git pull secrets
		for _, namespace := range namespaces.Items {
			currentGitSecret, err := c.GetSecret(ctx, namespace.Name, config.ZarfGitServerSecretName)
			if err != nil {
				continue
			}

			// Check if this is a Zarf managed secret or is in a namespace the Zarf agent will take action in
			if currentGitSecret.Labels[k8s.ZarfManagedByLabel] == "zarf" ||
				(namespace.Labels[k8s.AgentLabel] != "skip" && namespace.Labels[k8s.AgentLabel] != "ignore") {
				spinner.Updatef("Updating existing Zarf-managed git secret for namespace: '%s'", namespace.Name)

				// Create the secret
				newGitSecret := c.GenerateGitPullCreds(namespace.Name, config.ZarfGitServerSecretName, state.GitServer)
				if !reflect.DeepEqual(currentGitSecret.StringData, newGitSecret.StringData) {
					// Create or update the zarf git secret
					if _, err := c.CreateOrUpdateSecret(ctx, newGitSecret); err != nil {
						message.WarnErrf(err, "Problem creating git server secret for the %s namespace", namespace.Name)
					}
				}
			}
		}
		spinner.Success()
	}
}

// GetServiceInfoFromRegistryAddress gets the service info for a registry address if it is a NodePort
func (c *Cluster) GetServiceInfoFromRegistryAddress(ctx context.Context, stateRegistryAddress string) (string, error) {
	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	// If this is an internal service then we need to look it up and
	svc, port, err := serviceInfoFromNodePortURL(serviceList.Items, stateRegistryAddress)
	if err != nil {
		message.Debugf("registry appears to not be a nodeport service, using original address %q", stateRegistryAddress)
		return stateRegistryAddress, nil
	}

	return fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port), nil
}
