// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"slices"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fatih/color"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Zarf Cluster Constants.
const (
	ZarfNamespaceName       = "zarf"
	ZarfStateSecretName     = "zarf-state"
	ZarfStateDataKey        = "state"
	ZarfPackageInfoLabel    = "package-deploy-info"
	ZarfInitPackageInfoName = "zarf-package-init"
)

// InitZarfState initializes the Zarf state with the given temporary directory and init configs.
func (c *Cluster) InitZarfState(ctx context.Context, initOptions types.ZarfInitOptions) error {
	var (
		distro string
		err    error
	)

	spinner := message.NewProgressSpinner("Gathering cluster state information")
	defer spinner.Stop()

	// Attempt to load an existing state prior to init.
	// NOTE: We are ignoring the error here because we don't really expect a state to exist yet.
	spinner.Updatef("Checking cluster for existing Zarf deployment")
	state, _ := c.LoadZarfState(ctx)

	// If state is nil, this is a new cluster.
	if state == nil {
		state = &types.ZarfState{}
		spinner.Updatef("New cluster, no prior Zarf deployments found")

		// If the K3s component is being deployed, skip distro detection.
		if initOptions.ApplianceMode {
			distro = k8s.DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type.
			distro, err = c.DetectDistro(ctx)
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
		if state.LoggingSecret, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
			return fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
		}

		// Setup zarf agent PKI
		state.AgentTLS = pki.GeneratePKI(config.ZarfAgentHost)

		namespaces, err := c.GetNamespaces(ctx)
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
			namespaceCopy := namespace
			if _, err = c.UpdateNamespace(ctx, &namespaceCopy); err != nil {
				// This is not a hard failure, but we should log it.
				message.WarnErrf(err, "Unable to mark the namespace %s as ignored by Zarf Agent", namespace.Name)
			}
		}

		// Try to create the zarf namespace.
		spinner.Updatef("Creating the Zarf namespace")
		zarfNamespace := c.NewZarfManagedNamespace(ZarfNamespaceName)
		if _, err := c.CreateNamespace(ctx, zarfNamespace); err != nil {
			return fmt.Errorf("unable to create the zarf namespace: %w", err)
		}

		// Wait up to 2 minutes for the default service account to be created.
		// Some clusters seem to take a while to create this, see https://github.com/kubernetes/kubernetes/issues/66689.
		// The default SA is required for pods to start properly.
		saCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if _, err := c.WaitForServiceAccount(saCtx, ZarfNamespaceName, "default"); err != nil {
			return fmt.Errorf("unable get default Zarf service account: %w", err)
		}

		err = initOptions.GitServer.FillInEmptyValues()
		if err != nil {
			return err
		}
		state.GitServer = initOptions.GitServer
		err = initOptions.RegistryInfo.FillInEmptyValues()
		if err != nil {
			return err
		}
		state.RegistryInfo = initOptions.RegistryInfo
		initOptions.ArtifactServer.FillInEmptyValues()
		state.ArtifactServer = initOptions.ArtifactServer
	} else {
		if helpers.IsNotZeroAndNotEqual(initOptions.GitServer, state.GitServer) {
			message.Warn("Detected a change in Git Server init options on a re-init. Ignoring... To update run:")
			message.ZarfCommand("tools update-creds git")
		}
		if helpers.IsNotZeroAndNotEqual(initOptions.RegistryInfo, state.RegistryInfo) {
			message.Warn("Detected a change in Image Registry init options on a re-init. Ignoring... To update run:")
			message.ZarfCommand("tools update-creds registry")
		}
		if helpers.IsNotZeroAndNotEqual(initOptions.ArtifactServer, state.ArtifactServer) {
			message.Warn("Detected a change in Artifact Server init options on a re-init. Ignoring... To update run:")
			message.ZarfCommand("tools update-creds artifact")
		}
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

	spinner.Success()

	// Save the state back to K8s
	if err := c.SaveZarfState(ctx, state); err != nil {
		return fmt.Errorf("unable to save the Zarf state: %w", err)
	}

	return nil
}

// LoadZarfState returns the current zarf/zarf-state secret data or an empty ZarfState.
func (c *Cluster) LoadZarfState(ctx context.Context) (state *types.ZarfState, err error) {
	// Set up the API connection
	secret, err := c.GetSecret(ctx, ZarfNamespaceName, ZarfStateSecretName)
	if err != nil {
		return nil, fmt.Errorf("%w. %s", err, message.ColorWrap("Did you remember to zarf init?", color.Bold))
	}

	err = json.Unmarshal(secret.Data[ZarfStateDataKey], &state)
	if err != nil {
		return nil, err
	}

	c.debugPrintZarfState(state)

	return state, nil
}

func (c *Cluster) sanitizeZarfState(state *types.ZarfState) *types.ZarfState {
	// Overwrite the AgentTLS information
	state.AgentTLS.CA = []byte("**sanitized**")
	state.AgentTLS.Cert = []byte("**sanitized**")
	state.AgentTLS.Key = []byte("**sanitized**")

	// Overwrite the GitServer passwords
	state.GitServer.PushPassword = "**sanitized**"
	state.GitServer.PullPassword = "**sanitized**"

	// Overwrite the RegistryInfo passwords
	state.RegistryInfo.PushPassword = "**sanitized**"
	state.RegistryInfo.PullPassword = "**sanitized**"
	state.RegistryInfo.Secret = "**sanitized**"

	// Overwrite the ArtifactServer secret
	state.ArtifactServer.PushToken = "**sanitized**"

	// Overwrite the Logging secret
	state.LoggingSecret = "**sanitized**"

	return state
}

func (c *Cluster) debugPrintZarfState(state *types.ZarfState) {
	if state == nil {
		return
	}
	// this is a shallow copy, nested pointers WILL NOT be copied
	oldState := *state
	sanitized := c.sanitizeZarfState(&oldState)
	message.Debugf("ZarfState - %s", message.JSONValue(sanitized))
}

// SaveZarfState takes a given state and persists it to the Zarf/zarf-state secret.
func (c *Cluster) SaveZarfState(ctx context.Context, state *types.ZarfState) error {
	c.debugPrintZarfState(state)

	// Convert the data back to JSON.
	data, err := json.Marshal(&state)
	if err != nil {
		return err
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
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: dataWrapper,
	}

	// Attempt to create or update the secret and return.
	if _, err := c.CreateOrUpdateSecret(ctx, secret); err != nil {
		return fmt.Errorf("unable to create the zarf state secret")
	}

	return nil
}

// MergeZarfState merges init options for provided services into the provided state to create a new state struct
func MergeZarfState(oldState *types.ZarfState, initOptions types.ZarfInitOptions, services []string) (*types.ZarfState, error) {
	newState := *oldState
	var err error
	if slices.Contains(services, message.RegistryKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.RegistryInfo = helpers.MergeNonZero(newState.RegistryInfo, initOptions.RegistryInfo)
		// Set the state of the internal registry if it has changed
		if newState.RegistryInfo.Address == fmt.Sprintf("%s:%d", helpers.IPV4Localhost, newState.RegistryInfo.NodePort) {
			newState.RegistryInfo.InternalRegistry = true
		} else {
			newState.RegistryInfo.InternalRegistry = false
		}

		// Set the new passwords if they should be autogenerated
		if newState.RegistryInfo.PushPassword == oldState.RegistryInfo.PushPassword && oldState.RegistryInfo.InternalRegistry {
			if newState.RegistryInfo.PushPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.RegistryInfo.PullPassword == oldState.RegistryInfo.PullPassword && oldState.RegistryInfo.InternalRegistry {
			if newState.RegistryInfo.PullPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(services, message.GitKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.GitServer = helpers.MergeNonZero(newState.GitServer, initOptions.GitServer)

		// Set the state of the internal git server if it has changed
		if newState.GitServer.Address == types.ZarfInClusterGitServiceURL {
			newState.GitServer.InternalServer = true
		} else {
			newState.GitServer.InternalServer = false
		}

		// Set the new passwords if they should be autogenerated
		if newState.GitServer.PushPassword == oldState.GitServer.PushPassword && oldState.GitServer.InternalServer {
			if newState.GitServer.PushPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.GitServer.PullPassword == oldState.GitServer.PullPassword && oldState.GitServer.InternalServer {
			if newState.GitServer.PullPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(services, message.ArtifactKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, initOptions.ArtifactServer)

		// Set the state of the internal artifact server if it has changed
		if newState.ArtifactServer.Address == types.ZarfInClusterArtifactServiceURL {
			newState.ArtifactServer.InternalServer = true
		} else {
			newState.ArtifactServer.InternalServer = false
		}

		// Set an empty token if it should be autogenerated
		if newState.ArtifactServer.PushToken == oldState.ArtifactServer.PushToken && oldState.ArtifactServer.InternalServer {
			newState.ArtifactServer.PushToken = ""
		}
	}
	if slices.Contains(services, message.AgentKey) {
		newState.AgentTLS = pki.GeneratePKI(config.ZarfAgentHost)
	}

	return &newState, nil
}
