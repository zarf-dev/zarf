// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/types"
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
	l := logger.From(ctx)

	// Attempt to load an existing state prior to init.
	// NOTE: We are ignoring the error here because we don't really expect a state to exist yet.
	l.Debug("checking cluster for existing Zarf deployment")
	state, err := c.LoadZarfState(ctx)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing state: %w", err)
	}

	// If state is nil, this is a new cluster.
	if state == nil {
		state = &types.ZarfState{}
		l.Debug("new cluster, no prior Zarf deployments found")
		if initOptions.ApplianceMode {
			// If the K3s component is being deployed, skip distro detection.
			state.Distro = DistroIsK3s
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
			state.Distro = detectDistro(nodeList.Items[0], namespaceList.Items)
		}

		if state.Distro != DistroIsUnknown {
			l.Debug("Detected K8s distro", "name", state.Distro)
		}

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
			if namespace.Name == "zarf" {
				continue
			}
			l.Debug("marking namespace as ignored by Zarf Agent", "name", namespace.Name)

			if namespace.Labels == nil {
				// Ensure label map exists to avoid nil panic
				namespace.Labels = make(map[string]string)
			}
			// This label will tell the Zarf Agent to ignore this namespace.
			namespace.Labels[AgentLabel] = "ignore"
			namespaceCopy := namespace
			_, err := c.Clientset.CoreV1().Namespaces().Update(ctx, &namespaceCopy, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("unable to mark the namespace %s as ignored by Zarf Agent: %w", namespace.Name, err)
			}
		}

		// Try to create the zarf namespace.
		l.Debug("creating the Zarf namespace")
		zarfNamespace := NewZarfManagedApplyNamespace(ZarfNamespaceName)
		_, err = c.Clientset.CoreV1().Namespaces().Apply(ctx, zarfNamespace, metav1.ApplyOptions{FieldManager: FieldManagerName, Force: true})
		if err != nil {
			return fmt.Errorf("unable to apply the Zarf namespace: %w", err)
		}

		// Wait up to 2 minutes for the default service account to be created.
		// Some clusters seem to take a while to create this, see https://github.com/kubernetes/kubernetes/issues/66689.
		// The default SA is required for pods to start properly.
		saCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		err = retry.Do(func() error {
			_, err := c.Clientset.CoreV1().ServiceAccounts(ZarfNamespaceName).Get(saCtx, "default", metav1.GetOptions{})
			if err != nil {
				return err
			}
			return nil
		}, retry.Context(saCtx), retry.Attempts(0), retry.DelayType(retry.FixedDelay), retry.Delay(time.Second))
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
		// TODO (@austinabro321) validate immediately in `zarf init` if these are set and not equal and error out if so
		if helpers.IsNotZeroAndNotEqual(initOptions.GitServer, state.GitServer) {
			l.Warn("ignoring change in git sever init options on re-init, to update run `zarf tools update-creds git`")
		}
		if helpers.IsNotZeroAndNotEqual(initOptions.RegistryInfo, state.RegistryInfo) {
			l.Warn("ignoring change to registry init options on re-init, to update run `zarf tools update-creds registry`")
		}
		if helpers.IsNotZeroAndNotEqual(initOptions.ArtifactServer, state.ArtifactServer) {
			l.Warn("ignoring change to registry init options on re-init, to update run `zarf tools update-creds registry`")
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

	// Save the state back to K8s
	if err := c.SaveZarfState(ctx, state); err != nil {
		return fmt.Errorf("unable to save the Zarf state: %w", err)
	}

	return nil
}

// LoadZarfState returns the current zarf/zarf-state secret data or an empty ZarfState.
func (c *Cluster) LoadZarfState(ctx context.Context) (*types.ZarfState, error) {
	stateErr := errors.New("failed to load the Zarf State from the cluster, has Zarf been initiated?")
	secret, err := c.Clientset.CoreV1().Secrets(ZarfNamespaceName).Get(ctx, ZarfStateSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", stateErr, err)
	}

	state := &types.ZarfState{}
	err = json.Unmarshal(secret.Data[ZarfStateDataKey], &state)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", stateErr, err)
	}
	c.debugPrintZarfState(ctx, state)
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

func (c *Cluster) debugPrintZarfState(ctx context.Context, state *types.ZarfState) {
	if state == nil {
		return
	}
	// this is a shallow copy, nested pointers WILL NOT be copied
	oldState := *state
	sanitized := c.sanitizeZarfState(&oldState)
	logger.From(ctx).Debug("cluster.debugPrintZarfState", "state", sanitized)
}

// SaveZarfState takes a given state and persists it to the Zarf/zarf-state secret.
func (c *Cluster) SaveZarfState(ctx context.Context, state *types.ZarfState) error {
	c.debugPrintZarfState(ctx, state)

	data, err := json.Marshal(&state)
	if err != nil {
		return err
	}
	secret := v1ac.Secret(ZarfStateSecretName, ZarfNamespaceName).
		WithLabels(map[string]string{
			ZarfManagedByLabel: "zarf",
		}).
		WithType(corev1.SecretTypeOpaque).
		WithData(map[string][]byte{
			ZarfStateDataKey: data,
		})

	_, err = c.Clientset.CoreV1().Secrets(*secret.Namespace).Apply(ctx, secret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return fmt.Errorf("unable to apply the zarf state secret: %w", err)
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

		// Set the new passwords if they should be autogenerated
		if newState.RegistryInfo.PushPassword == oldState.RegistryInfo.PushPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PushPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.RegistryInfo.PullPassword == oldState.RegistryInfo.PullPassword && oldState.RegistryInfo.IsInternal() {
			if newState.RegistryInfo.PullPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(services, message.GitKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.GitServer = helpers.MergeNonZero(newState.GitServer, initOptions.GitServer)

		// Set the new passwords if they should be autogenerated
		if newState.GitServer.PushPassword == oldState.GitServer.PushPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PushPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
		if newState.GitServer.PullPassword == oldState.GitServer.PullPassword && oldState.GitServer.IsInternal() {
			if newState.GitServer.PullPassword, err = helpers.RandomString(types.ZarfGeneratedPasswordLen); err != nil {
				return nil, fmt.Errorf("%s: %w", lang.ErrUnableToGenerateRandomSecret, err)
			}
		}
	}
	if slices.Contains(services, message.ArtifactKey) {
		// TODO: Replace use of reflections with explicit setting
		newState.ArtifactServer = helpers.MergeNonZero(newState.ArtifactServer, initOptions.ArtifactServer)

		// Set an empty token if it should be autogenerated
		if newState.ArtifactServer.PushToken == oldState.ArtifactServer.PushToken && oldState.ArtifactServer.IsInternal() {
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
