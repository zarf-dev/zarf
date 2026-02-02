// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
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

// addRegistryAuthEntries adds registry authentication entries for a service's ClusterIP and DNS hostname.
func addRegistryAuthEntries(auths DockerConfigEntry, svc *corev1.Service, port int32, authValue string) {
	kubeDNSRegistryURL := net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprintf("%d", port))
	auths[kubeDNSRegistryURL] = DockerConfigEntryWithAuth{
		Auth: authValue,
	}

	kubeDNSRegistryHostname := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)
	auths[kubeDNSRegistryHostname] = DockerConfigEntryWithAuth{
		Auth: authValue,
	}
}

// GenerateRegistryPullCreds generates a secret containing the registry credentials.
func (c *Cluster) GenerateRegistryPullCreds(ctx context.Context, namespace, name string, registryInfo state.RegistryInfo) (*v1ac.SecretApplyConfiguration, error) {
	// Auth field must be username:password and base64 encoded
	fieldValue := registryInfo.PullUsername + ":" + registryInfo.PullPassword
	authEncodedValue := base64.StdEncoding.EncodeToString([]byte(fieldValue))

	dockerConfigJSON := DockerConfig{
		Auths: DockerConfigEntry{
			// nodePort for zarf-docker-registry - ie 127.0.0.1:31999
			registryInfo.Address: DockerConfigEntryWithAuth{
				Auth: authEncodedValue,
			},
		},
	}

	if registryInfo.RegistryMode == state.RegistryModeProxy {
		svc, err := c.Clientset.CoreV1().Services("zarf").Get(ctx, "zarf-docker-registry", metav1.GetOptions{})
		if err != nil && !kerrors.IsNotFound(err) {
			return nil, err
		}
		if !kerrors.IsNotFound(err) {
			if len(svc.Spec.Ports) == 0 {
				return nil, fmt.Errorf("registry service has no ports")
			}
			port := svc.Spec.Ports[0].Port
			addRegistryAuthEntries(dockerConfigJSON.Auths, svc, port, authEncodedValue)
		}
	} else {
		serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		// Build zarf-docker-registry service address and internal dns string
		svc, port, err := serviceInfoFromNodePortURL(serviceList.Items, registryInfo.Address)
		if err == nil {
			addRegistryAuthEntries(dockerConfigJSON.Auths, &svc, int32(port), authEncodedValue)
		}
	}

	// Convert to JSON
	dockerConfigData, err := json.Marshal(dockerConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal the .dockerconfigjson secret data for the image pull secret: %w", err)
	}

	secretDockerConfig := v1ac.Secret(name, namespace).
		WithLabels(map[string]string{
			state.ZarfManagedByLabel: "zarf",
		}).
		WithType(corev1.SecretTypeDockerConfigJson).
		WithData(map[string][]byte{
			".dockerconfigjson": dockerConfigData,
		})

	return secretDockerConfig, nil
}

// GenerateGitPullCreds generates a secret containing the git credentials.
func (c *Cluster) GenerateGitPullCreds(namespace, name string, gitServerInfo state.GitServerInfo) *v1ac.SecretApplyConfiguration {
	return v1ac.Secret(name, namespace).
		WithLabels(map[string]string{
			state.ZarfManagedByLabel: "zarf",
		}).WithType(corev1.SecretTypeOpaque).
		WithStringData(map[string]string{
			"username": gitServerInfo.PullUsername,
			"password": gitServerInfo.PullPassword,
		})
}

// UpdateZarfManagedImageSecrets updates all Zarf-managed image secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedImageSecrets(ctx context.Context, s *state.State) error {
	l := logger.From(ctx)

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
		if currentRegistrySecret.Labels[state.ZarfManagedByLabel] != "zarf" && (namespace.Labels[AgentLabel] == "skip" || namespace.Labels[AgentLabel] == "ignore") {
			continue
		}
		newRegistrySecret, err := c.GenerateRegistryPullCreds(ctx, namespace.Name, config.ZarfImagePullSecretName, s.RegistryInfo)
		if err != nil {
			return err
		}
		l.Info("applying Zarf managed registry secret for namespace", "name", namespace.Name)
		_, err = c.Clientset.CoreV1().Secrets(*newRegistrySecret.Namespace).Apply(ctx, newRegistrySecret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateZarfManagedGitSecrets updates all Zarf-managed git secrets in all namespaces based on state
func (c *Cluster) UpdateZarfManagedGitSecrets(ctx context.Context, s *state.State) error {
	l := logger.From(ctx)

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
		if currentGitSecret.Labels[state.ZarfManagedByLabel] != "zarf" && (namespace.Labels[AgentLabel] == "skip" || namespace.Labels[AgentLabel] == "ignore") {
			continue
		}
		newGitSecret := c.GenerateGitPullCreds(namespace.Name, config.ZarfGitServerSecretName, s.GitServer)
		l.Info("applying Zarf managed git secret for namespace", "name", namespace.Name)
		_, err = c.Clientset.CoreV1().Secrets(*newGitSecret.Namespace).Apply(ctx, newGitSecret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyZarfManagedMTLSSecrets regenerates and updates all Zarf-managed mTLS secrets.
// It generates fresh certificates, applies them to the zarf namespace, and copies
// the client certificate to all namespaces that have the mTLS client secret.
func (c *Cluster) ApplyZarfManagedMTLSSecrets(ctx context.Context) error {
	l := logger.From(ctx)

	serverPKI, clientPKI, err := pki.GenerateMTLSCerts(
		state.ZarfRegistryMTLSCASubject,
		state.ZarfRegistryMTLSServerHosts,
		state.ZarfRegistryMTLSServerCommonName,
		state.ZarfRegistryMTLSClientCommonName,
	)
	if err != nil {
		return fmt.Errorf("failed to generate mTLS certificates: %w", err)
	}

	if err := c.ApplyZarfRegistryCertSecrets(ctx, serverPKI, clientPKI); err != nil {
		return fmt.Errorf("failed to apply registry certs to zarf namespace: %w", err)
	}

	namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: state.ZarfManagedByLabel + "=zarf",
	})
	if err != nil {
		return fmt.Errorf("failed to list Zarf-managed namespaces: %w", err)
	}

	for _, namespace := range namespaceList.Items {
		// Skip the zarf namespace since we already updated it
		if namespace.Name == state.ZarfNamespaceName {
			continue
		}
		// Skip if namespace is skipped/ignored and secret is not managed by Zarf
		if namespace.Labels[AgentLabel] == "skip" || namespace.Labels[AgentLabel] == "ignore" {
			continue
		}
		l.Info("applying Zarf managed mTLS client secret for namespace", "name", namespace.Name)
		if err := c.ApplyRegistryClientCertSecret(ctx, clientPKI, namespace.Name); err != nil {
			return err
		}
	}

	return nil
}

// GetServiceInfoFromRegistryAddress gets the service info for a registry address
// If the address is not a service then it is returned
// If the address is a service then the service DNS name and clusterIP is returned
func (c *Cluster) GetServiceInfoFromRegistryAddress(ctx context.Context, registryInfo state.RegistryInfo) (string, string, error) {
	if registryInfo.RegistryMode == state.RegistryModeProxy {
		svc, err := c.Clientset.CoreV1().Services(state.ZarfNamespaceName).Get(ctx, ZarfRegistryName, metav1.GetOptions{})
		if err != nil {
			return "", "", err
		}
		if len(svc.Spec.Ports) == 0 {
			return "", "", fmt.Errorf("registry service has no ports")
		}
		serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local:%d", ZarfRegistryName, state.ZarfNamespaceName, svc.Spec.Ports[0].Port)
		clusterIP := net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprintf("%d", svc.Spec.Ports[0].Port))
		return serviceDNS, clusterIP, nil
	}

	serviceList, err := c.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}

	// If this is an internal service then we need to look it up and
	svc, port, err := serviceInfoFromNodePortURL(serviceList.Items, registryInfo.Address)
	if err != nil {
		logger.From(ctx).Debug("registry appears to not be a nodeport service, using original address", "address", registryInfo.Address)
		return registryInfo.Address, "", nil
	}

	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)
	clusterIP := net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprintf("%d", port))
	return serviceDNS, clusterIP, nil
}
