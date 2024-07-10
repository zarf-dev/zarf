// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/fatih/color"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/pki"
	"github.com/defenseunicorns/zarf/src/types"
)

// Zarf Cluster Constants.
const (
	ZarfManagedByLabel   = "app.kubernetes.io/managed-by"
	ZarfNamespaceName    = "zarf"
	ZarfStateSecretName  = "zarf-state"
	ZarfStateDataKey     = "state"
	ZarfPackageInfoLabel = "package-deploy-info"
)

// InitZarfState initializes the Zarf state with the given temporary directory and init configs.
func (c *Cluster) InitZarfState(ctx context.Context, initOptions types.ZarfInitOptions) error {
	var distro string

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

		if initOptions.ApplianceMode {
			// If the K3s component is being deployed, skip distro detection.
			distro = DistroIsK3s
			state.ZarfAppliance = true
		} else {
			// Otherwise, trying to detect the K8s distro type.
			nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			if len(nodeList.Items) == 0 {
				return fmt.Errorf("cannot init Zarf state in empty cluster")
			}
			namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			distro = detectDistro(nodeList.Items[0], namespaceList.Items)
		}

		if distro != DistroIsUnknown {
			spinner.Updatef("Detected K8s distro %s", distro)
		}

		// Defaults
		state.Distro = distro

		// Setup zarf agent PKI
		agentTLS, err := pki.GeneratePKI(config.ZarfAgentHost)
		if err != nil {
			return err
		}
		state.AgentTLS = agentTLS

		namespaceList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("unable to get the Kubernetes namespaces: %w", err)
		}
		// Mark existing namespaces as ignored for the zarf agent to prevent mutating resources we don't own.
		for _, namespace := range namespaceList.Items {
			spinner.Updatef("Marking existing namespace %s as ignored by Zarf Agent", namespace.Name)
			if namespace.Labels == nil {
				// Ensure label map exists to avoid nil panic
				namespace.Labels = make(map[string]string)
			}
			// This label will tell the Zarf Agent to ignore this namespace.
			namespace.Labels[AgentLabel] = "ignore"
			namespaceCopy := namespace
			_, err := c.Clientset.CoreV1().Namespaces().Update(ctx, &namespaceCopy, metav1.UpdateOptions{})
			if err != nil {
				// This is not a hard failure, but we should log it.
				message.WarnErrf(err, "Unable to mark the namespace %s as ignored by Zarf Agent", namespace.Name)
			}
		}

		// Try to create the zarf namespace.
		spinner.Updatef("Creating the Zarf namespace")
		zarfNamespace := NewZarfManagedNamespace(ZarfNamespaceName)
		err = func() error {
			_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, zarfNamespace, metav1.CreateOptions{})
			if err != nil && !kerrors.IsAlreadyExists(err) {
				return fmt.Errorf("unable to create the Zarf namespace: %w", err)
			}
			if err == nil {
				return nil
			}
			_, err = c.Clientset.CoreV1().Namespaces().Update(ctx, zarfNamespace, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("unable to update the Zarf namespace: %w", err)
			}
			return nil
		}()
		if err != nil {
			return err
		}

		// Wait up to 2 minutes for the default service account to be created.
		// Some clusters seem to take a while to create this, see https://github.com/kubernetes/kubernetes/issues/66689.
		// The default SA is required for pods to start properly.
		saCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		err = func(ctx context.Context, ns, name string) error {
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("failed to get service account %s/%s: %w", ns, name, ctx.Err())
				case <-timer.C:
					_, err := c.Clientset.CoreV1().ServiceAccounts(ns).Get(ctx, name, metav1.GetOptions{})
					if err != nil && !kerrors.IsNotFound(err) {
						return err
					}
					if kerrors.IsNotFound(err) {
						message.Debug("Service account %s/%s not found, retrying...", ns, name)
						timer.Reset(1 * time.Second)
						continue
					}
					return nil
				}
			}
		}(saCtx, ZarfNamespaceName, "default")
		if err != nil {
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
	case DistroIsK3s, DistroIsK3d:
		state.StorageClass = "local-path"

	case DistroIsKind, DistroIsGKE:
		state.StorageClass = "standard"

	case DistroIsDockerDesktop:
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
	secret, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).Get(ctx, ZarfStateSecretName, metav1.GetOptions{})
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

	return state
}

func (c *Cluster) debugPrintZarfState(state *types.ZarfState) {
	if state == nil {
		return
	}
	// this is a shallow copy, nested pointers WILL NOT be copied
	oldState := *state
	sanitized := c.sanitizeZarfState(&oldState)
	b, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return
	}
	message.Debugf("ZarfState - %s", string(b))
}

// SaveZarfState takes a given state and persists it to the Zarf/zarf-state secret.
func (c *Cluster) SaveZarfState(ctx context.Context, state *types.ZarfState) error {
	c.debugPrintZarfState(state)

	data, err := json.Marshal(&state)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ZarfStateSecretName,
			Namespace: ZarfNamespaceName,
			Labels: map[string]string{
				ZarfManagedByLabel: "zarf",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			ZarfStateDataKey: data,
		},
	}

	// Attempt to create or update the secret and return.
	_, err = c.Clientset.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !kerrors.IsAlreadyExists(err) {
		return fmt.Errorf("unable to create the zarf state secret: %w", err)
	}
	if err == nil {
		return nil
	}
	_, err = c.Clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("unable to update the zarf state secret: %w", err)
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
		// TODO: Internal registry should be a function of the address and not a property.
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
		// TODO: Internal server should be a function of the address and not a property.
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
		// TODO: Internal server should be a function of the address and not a property.
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
		agentTLS, err := pki.GeneratePKI(config.ZarfAgentHost)
		if err != nil {
			return nil, err
		}
		newState.AgentTLS = agentTLS
	}

	return &newState, nil
}
