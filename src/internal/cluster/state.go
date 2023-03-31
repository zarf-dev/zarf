// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Zarf Cluster Constants.
const (
	ZarfNamespace        = "zarf"
	ZarfStateSecretName  = "zarf-state"
	ZarfStateDataKey     = "state"
	ZarfPackageInfoLabel = "package-deploy-info"
)

// InitZarfState initializes the Zarf state with the given temporary directory and init configs.
func (c *Cluster) InitZarfState(initOptions types.ZarfInitOptions) error {
	var (
		clusterArch string
		distro      string
		err         error
	)

	spinner := message.NewProgressSpinner("Gathering cluster information")
	defer spinner.Stop()

	spinner.Updatef("Getting cluster architecture")
	if clusterArch, err = c.Kube.GetArchitecture(); err != nil {
		spinner.Errorf(err, "Unable to validate the cluster system architecture")
	}

	// Attempt to load an existing state prior to init.
	// NOTE: We are ignoring the error here because we don't really expect a state to exist yet.
	spinner.Updatef("Checking cluster for existing Zarf deployment")
	state, _ := c.LoadZarfState()

	// If the distro isn't populated in the state, assume this is a new cluster.
	if state.Distro == "" {
		spinner.Updatef("New cluster, no prior Zarf deployments found")

		// If the K3s component is being deployed, skip distro detection.
		if initOptions.ApplianceMode {
			distro = k8s.DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type.
			distro, err = c.Kube.DetectDistro()
			if err != nil {
				// This is a basic failure right now but likely could be polished to provide user guidance to resolve.
				return fmt.Errorf("unable to connect to the cluster to verify the distro: %w", err)
			}
		}

		if distro != k8s.DistroIsUnknown {
			spinner.Updatef("Detected K8s distro %s", distro)
		}

		// Defaults
		state.Distro = distro
		state.Architecture = clusterArch
		state.LoggingSecret = utils.RandomString(config.ZarfGeneratedPasswordLen)

		// Setup zarf agent PKI
		state.AgentTLS = pki.GeneratePKI(config.ZarfAgentHost)

		namespaces, err := c.Kube.GetNamespaces()
		if err != nil {
			return fmt.Errorf("unable to get the Kubernetes namespaces: %w", err)
		}
		// Mark existing namespaces as ignored for the zarf agent to prevent mutating resources we don't own.
		for _, namespace := range namespaces.Items {
			spinner.Updatef("Marking existing namespace %s as ignored by Zarf Agent", namespace.Name)
			if namespace.Labels == nil {
				// Ensure label map exists to avoid nil panic
				namespace.Labels = make(map[string]string)
			}
			// This label will tell the Zarf Agent to ignore this namespace.
			namespace.Labels[agentLabel] = "ignore"
			if _, err = c.Kube.UpdateNamespace(&namespace); err != nil {
				// This is not a hard failure, but we should log it.
				message.Errorf(err, "Unable to mark the namespace %s as ignored by Zarf Agent", namespace.Name)
			}
		}

		// Try to create the zarf namespace.
		spinner.Updatef("Creating the Zarf namespace")
		if _, err := c.Kube.CreateNamespace(ZarfNamespace, nil); err != nil {
			return fmt.Errorf("unable to create the zarf namespace: %w", err)
		}

		// Wait up to 2 minutes for the default service account to be created.
		// Some clusters seem to take a while to create this, see https://github.com/kubernetes/kubernetes/issues/66689.
		// The default SA is required for pods to start properly.
		if _, err := c.Kube.WaitForServiceAccount(ZarfNamespace, "default", 2*time.Minute); err != nil {
			return fmt.Errorf("unable get default Zarf service account: %w", err)
		}
	}

	if clusterArch != state.Architecture {
		return fmt.Errorf("cluster architecture %s does not match the Zarf state architecture %s", clusterArch, state.Architecture)
	}

	switch state.Distro {
	case k8s.DistroIsK3s, k8s.DistroIsK3d:
		state.StorageClass = "local-path"

	case k8s.DistroIsKind, k8s.DistroIsGKE:
		state.StorageClass = "standard"

	case k8s.DistroIsDockerDesktop:
		state.StorageClass = "hostpath"
	}

	if initOptions.StorageClass != "" {
		state.StorageClass = initOptions.StorageClass
	}

	state.GitServer = c.fillInEmptyGitServerValues(initOptions.GitServer)
	state.RegistryInfo = c.fillInEmptyContainerRegistryValues(initOptions.RegistryInfo)

	spinner.Success()

	// Save the state back to K8s
	if err := c.SaveZarfState(state); err != nil {
		return fmt.Errorf("unable to save the Zarf state: %w", err)
	}

	return nil
}

// LoadZarfState returns the current zarf/zarf-state secret data or an empty ZarfState.
func (c *Cluster) LoadZarfState() (types.ZarfState, error) {
	message.Debug("k8s.LoadZarfState()")

	// The empty state that we will try to fill.
	state := types.ZarfState{}

	// Set up the API connection
	secret, err := c.Kube.GetSecret(ZarfNamespace, ZarfStateSecretName)
	if err != nil {
		return state, err
	}

	_ = json.Unmarshal(secret.Data[ZarfStateDataKey], &state)

	message.Debugf("ZarfState = %s", message.JSONValue(c.sanitizeZarfState(state)))

	return state, nil
}

func (c *Cluster) sanitizeZarfState(state types.ZarfState) types.ZarfState {
	sanitizedState := state

	// Overwrite the AgentTLS information
	sanitizedState.AgentTLS.CA = []byte("**sanitized**")
	sanitizedState.AgentTLS.Cert = []byte("**sanitized**")
	sanitizedState.AgentTLS.Key = []byte("**sanitized**")

	// Overwrite the GitServer passwords
	sanitizedState.GitServer.PushPassword = "**sanitized**"
	sanitizedState.GitServer.PullPassword = "**sanitized**"

	// Overwrite the RegistryInfo passwords
	sanitizedState.RegistryInfo.PushPassword = "**sanitized**"
	sanitizedState.RegistryInfo.PullPassword = "**sanitized**"
	sanitizedState.RegistryInfo.Secret = "**sanitized**"

	// Overwrite the Logging secret
	sanitizedState.LoggingSecret = "**sanitized**"

	return sanitizedState
}

// SaveZarfState takes a given state and persists it to the Zarf/zarf-state secret.
func (c *Cluster) SaveZarfState(state types.ZarfState) error {
	message.Debugf("k8s.SaveZarfState()")
	message.Debug(message.JSONValue(state))

	// Convert the data back to JSON.
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("unable to json-encode the zarf state")
	}

	// Set up the data wrapper.
	dataWrapper := make(map[string][]byte)
	dataWrapper[ZarfStateDataKey] = data

	// The secret object.
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ZarfStateSecretName,
			Namespace: ZarfNamespace,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: dataWrapper,
	}

	// Attempt to create or update the secret and return.
	if err := c.Kube.CreateOrUpdateSecret(secret); err != nil {
		return fmt.Errorf("unable to create the zarf state secret")
	}

	return nil
}

func (c *Cluster) fillInEmptyContainerRegistryValues(containerRegistry types.RegistryInfo) types.RegistryInfo {
	// Set default NodePort if none was provided
	if containerRegistry.NodePort == 0 {
		containerRegistry.NodePort = config.ZarfInClusterContainerRegistryNodePort
	}

	// Set default url if an external registry was not provided
	if containerRegistry.Address == "" {
		containerRegistry.InternalRegistry = true
		containerRegistry.Address = fmt.Sprintf("%s:%d", config.IPV4Localhost, containerRegistry.NodePort)
	}

	// Generate a push-user password if not provided by init flag
	if containerRegistry.PushPassword == "" {
		containerRegistry.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
	}

	// Set pull-username if not provided by init flag
	if containerRegistry.PullUsername == "" {
		if containerRegistry.InternalRegistry {
			containerRegistry.PullUsername = config.ZarfRegistryPullUser
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			containerRegistry.PullUsername = containerRegistry.PushUsername
		}
	}
	if containerRegistry.PullPassword == "" {
		if containerRegistry.InternalRegistry {
			containerRegistry.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
		} else {
			// If this is an external registry and a pull-user wasn't provided, use the same credentials as the push user
			containerRegistry.PullPassword = containerRegistry.PushPassword
		}
	}

	if containerRegistry.Secret == "" {
		containerRegistry.Secret = utils.RandomString(config.ZarfGeneratedSecretLen)
	}

	return containerRegistry
}

// Fill in empty GitServerInfo values with the defaults.
func (c *Cluster) fillInEmptyGitServerValues(gitServer types.GitServerInfo) types.GitServerInfo {
	// Set default svc url if an external repository was not provided
	if gitServer.Address == "" {
		gitServer.Address = config.ZarfInClusterGitServiceURL
		gitServer.InternalServer = true
	}

	// Generate a push-user password if not provided by init flag
	if gitServer.PushPassword == "" {
		gitServer.PushPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
	}

	// Set read-user information if using an internal repository, otherwise copy from the push-user
	if gitServer.PullUsername == "" {
		if gitServer.InternalServer {
			gitServer.PullUsername = config.ZarfGitReadUser
		} else {
			gitServer.PullUsername = gitServer.PushUsername
		}
	}
	if gitServer.PullPassword == "" {
		if gitServer.InternalServer {
			gitServer.PullPassword = utils.RandomString(config.ZarfGeneratedPasswordLen)
		} else {
			gitServer.PullPassword = gitServer.PushPassword
		}
	}

	return gitServer
}
