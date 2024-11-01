// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
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
func (c *Cluster) GenerateRegistryPullCreds(ctx context.Context, namespace, name string, registryInfo types.RegistryInfo) (*v1ac.SecretApplyConfiguration, error) {
	// Auth field must be username:password and base64 encoded
	fieldValue := registryInfo.PullUsername + ":" + registryInfo.PullPassword
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	dockerConfigJSON := DockerConfig{
		Auths: DockerConfigEntry{
			// nodePort for zarf-docker-registry
			registryInfo.Address: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}

	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	// Build zarf-docker-registry service address string
	svc, port, err := serviceInfoFromNodePortURL(serviceList.Items, registryInfo.Address)
	if err == nil {
		kubeDNSRegistryURL := fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port)
		dockerConfigJSON.Auths[kubeDNSRegistryURL] = DockerConfigEntryWithAuth{
			Auth: authEncodedValue,
		}
	}

	// Convert to JSON
	dockerConfigData, err := json.Marshal(dockerConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal the .dockerconfigjson secret data for the image pull secret: %w", err)
	}

	secretDockerConfig := v1ac.Secret(name, namespace).
		WithLabels(map[string]string{
			ZarfManagedByLabel: "zarf",
		}).
		WithType(corev1.SecretTypeDockerConfigJson).
		WithData(map[string][]byte{
			".dockerconfigjson": dockerConfigData,
		})

	return secretDockerConfig, nil
}

// GenerateGitPullCreds generates a secret containing the git credentials.
func (c *Cluster) GenerateGitPullCreds(namespace, name string, gitServerInfo types.GitServerInfo) *v1ac.SecretApplyConfiguration {
	return v1ac.Secret(name, namespace).
		WithLabels(map[string]string{
			ZarfManagedByLabel: "zarf",
		}).WithType(corev1.SecretTypeOpaque).
		WithStringData(map[string]string{
			"username": gitServerInfo.PullUsername,
			"password": gitServerInfo.PullPassword,
		})
}

// UpdateZarfManagedImageSecrets updates all Zarf-managed image secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedImageSecrets(ctx context.Context, state *types.ZarfState) error {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed image secrets")
	defer spinner.Stop()

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	// Update all image pull secrets
	for _, namespace := range namespaceList.Items {
		currentRegistrySecret, err := c.Clientset.CoreV1().Secrets(namespace.Name).Get(ctx, config.ZarfImagePullSecretName, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		// Skip if namespace is skipped and secret is not managed by Zarf.
		if currentRegistrySecret.Labels[ZarfManagedByLabel] != "zarf" && (namespace.Labels[AgentLabel] == "skip" || namespace.Labels[AgentLabel] == "ignore") {
			continue
		}
		newRegistrySecret, err := c.GenerateRegistryPullCreds(ctx, namespace.Name, config.ZarfImagePullSecretName, state.RegistryInfo)
		if err != nil {
			return err
		}
		spinner.Updatef("Updating existing Zarf-managed image secret for namespace: '%s'", namespace.Name)
		_, err = c.Clientset.CoreV1().Secrets(*newRegistrySecret.Namespace).Apply(ctx, newRegistrySecret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}
	}

	spinner.Success()
	return nil
}

// UpdateZarfManagedGitSecrets updates all Zarf-managed git secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedGitSecrets(ctx context.Context, state *types.ZarfState) error {
	spinner := message.NewProgressSpinner("Updating existing Zarf-managed git secrets")
	defer spinner.Stop()

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, namespace := range namespaceList.Items {
		currentGitSecret, err := c.Clientset.CoreV1().Secrets(namespace.Name).Get(ctx, config.ZarfGitServerSecretName, metav1.GetOptions{})
		if kerrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			continue
		}
		// Skip if namespace is skipped and secret is not managed by Zarf.
		if currentGitSecret.Labels[ZarfManagedByLabel] != "zarf" && (namespace.Labels[AgentLabel] == "skip" || namespace.Labels[AgentLabel] == "ignore") {
			continue
		}
		newGitSecret := c.GenerateGitPullCreds(namespace.Name, config.ZarfGitServerSecretName, state.GitServer)
		spinner.Updatef("Updating existing Zarf-managed git secret for namespace: %s", namespace.Name)
		_, err = c.Clientset.CoreV1().Secrets(*newGitSecret.Namespace).Apply(ctx, newGitSecret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}
	}

	spinner.Success()
	return nil
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
