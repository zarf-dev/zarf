// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/types"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	// DefaultTimeout is the default time to wait for a cluster to be ready.
	DefaultTimeout = 30 * time.Second
	// AgentLabel is used to give instructions to the Zarf agent
	AgentLabel = "zarf.dev/agent"
	// FieldManagerName is the field manager used during server side apply
	FieldManagerName = "zarf"
)

// Cluster Zarf specific cluster management functions.
type Cluster struct {
	// Clientset implements k8s client api
	Clientset kubernetes.Interface
	// RestConfig holds common options for a k8s client
	RestConfig *rest.Config
	// Watcher implements kstatus StatusWatcher
	Watcher watcher.StatusWatcher
}

// NewWithWait creates a new Cluster instance and waits for the given timeout for the cluster to be ready.
func NewWithWait(ctx context.Context) (*Cluster, error) {
	start := time.Now()
	l := logger.From(ctx)
	l.Info("waiting for cluster connection")

	c, err := New(ctx)
	if err != nil {
		return nil, err
	}
	err = retry.Do(func() error {
		nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(nodeList.Items) < 1 {
			return fmt.Errorf("cluster does not have any nodes")
		}
		pods, err := c.Clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodRunning {
				return nil
			}
		}
		return fmt.Errorf("no pods are in succeeded or running state")
	}, retry.Context(ctx), retry.Attempts(0), retry.DelayType(retry.FixedDelay), retry.Delay(time.Second))
	if err != nil {
		return nil, err
	}

	l.Debug("done waiting for cluster, connected", "duration", time.Since(start))

	return c, nil
}

// New creates a new Cluster instance and validates connection to the cluster by fetching the Kubernetes version.
func New(_ context.Context) (*Cluster, error) {
	clusterErr := errors.New("unable to connect to the cluster")
	clientset, cfg, err := ClientAndConfig()
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	w, err := WatcherForConfig(cfg)
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	c := &Cluster{
		Clientset:  clientset,
		RestConfig: cfg,
		Watcher:    w,
	}
	// Dogsled the version output. We just want to ensure no errors were returned to validate cluster connection.
	_, err = c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	return c, nil
}

// ClientAndConfig returns a Kubernetes client and the rest config used to configure the client.
func ClientAndConfig() (kubernetes.Interface, *rest.Config, error) {
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	clientCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, nil)
	cfg, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return clientset, cfg, nil
}

// WatcherForConfig returns a status watcher for the give Kubernetes configuration.
func WatcherForConfig(cfg *rest.Config) (watcher.StatusWatcher, error) {
	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
	if err != nil {
		return nil, err
	}
	sw := watcher.NewDefaultStatusWatcher(dynamicClient, restMapper)
	return sw, nil
}

// InitStateOptions tracks the user-defined options during cluster initialization.
type InitStateOptions struct {
	// Indicates if Zarf was initialized while deploying its own k8s cluster
	ApplianceMode bool
	// Information about the repository Zarf is going to be using
	GitServer types.GitServerInfo
	// Information about the container registry Zarf is going to be using
	RegistryInfo types.RegistryInfo
	// Information about the artifact registry Zarf is going to be using
	ArtifactServer types.ArtifactServerInfo
	// StorageClass of the k8s cluster Zarf is initializing
	StorageClass string
}

// InitState takes initOptions and hydrates a cluster's state from InitStateOptions.
func (c *Cluster) InitState(ctx context.Context, opts InitStateOptions) error {
	l := logger.From(ctx)

	// Attempt to load an existing state prior to init.
	// NOTE: We are ignoring the error here because we don't really expect a state to exist yet.
	l.Debug("checking cluster for existing Zarf deployment")
	s, err := c.LoadState(ctx)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing state: %w", err)
	}

	// If state is nil, this is a new cluster.
	// TODO(mkcp): Simplify nesting with early returns closer to the top of the function.
	if s == nil {
		s = &state.State{}
		l.Debug("new cluster, no prior Zarf deployments found")
		if opts.ApplianceMode {
			// If the K3s component is being deployed, skip distro detection.
			s.Distro = DistroIsK3s
			s.ZarfAppliance = true
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
			s.Distro = detectDistro(nodeList.Items[0], namespaceList.Items)
		}

		if s.Distro != DistroIsUnknown {
			l.Debug("Detected K8s distro", "name", s.Distro)
		}

		// Setup zarf agent PKI
		agentTLS, err := pki.GeneratePKI(config.ZarfAgentHost)
		if err != nil {
			return err
		}
		s.AgentTLS = agentTLS

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
		zarfNamespace := NewZarfManagedApplyNamespace(state.ZarfNamespaceName)
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
			_, err := c.Clientset.CoreV1().ServiceAccounts(state.ZarfNamespaceName).Get(saCtx, "default", metav1.GetOptions{})
			if err != nil {
				return err
			}
			return nil
		}, retry.Context(saCtx), retry.Attempts(0), retry.DelayType(retry.FixedDelay), retry.Delay(time.Second))
		if err != nil {
			return fmt.Errorf("unable get default Zarf service account: %w", err)
		}

		err = opts.GitServer.FillInEmptyValues()
		if err != nil {
			return err
		}
		s.GitServer = opts.GitServer
		err = opts.RegistryInfo.FillInEmptyValues()
		if err != nil {
			return err
		}
		s.RegistryInfo = opts.RegistryInfo
		opts.ArtifactServer.FillInEmptyValues()
		s.ArtifactServer = opts.ArtifactServer
	} else {
		// TODO (@austinabro321) validate immediately in `zarf init` if these are set and not equal and error out if so
		if helpers.IsNotZeroAndNotEqual(opts.GitServer, s.GitServer) {
			l.Warn("ignoring change in git sever init options on re-init, to update run `zarf tools update-creds git`")
		}
		if helpers.IsNotZeroAndNotEqual(opts.RegistryInfo, s.RegistryInfo) {
			l.Warn("ignoring change to registry init options on re-init, to update run `zarf tools update-creds registry`")
		}
		if helpers.IsNotZeroAndNotEqual(opts.ArtifactServer, s.ArtifactServer) {
			l.Warn("ignoring change to registry init options on re-init, to update run `zarf tools update-creds registry`")
		}
	}

	switch s.Distro {
	case DistroIsK3s, DistroIsK3d:
		s.StorageClass = "local-path"

	case DistroIsKind, DistroIsGKE:
		s.StorageClass = "standard"

	case DistroIsDockerDesktop:
		s.StorageClass = "hostpath"
	}

	if opts.StorageClass != "" {
		s.StorageClass = opts.StorageClass
	}

	// Save the state back to K8s
	if err := c.SaveState(ctx, s); err != nil {
		return fmt.Errorf("unable to save the Zarf state: %w", err)
	}

	return nil
}

// LoadState utilizes the k8s Clientset to load and return the current state.State data or an empty state.State if no
// cluster is found.
func (c *Cluster) LoadState(ctx context.Context) (*state.State, error) {
	stateErr := errors.New("failed to load the Zarf State from the cluster, has Zarf been initiated")
	secret, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Get(ctx, state.ZarfStateSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", stateErr, err)
	}

	s := &state.State{}
	err = json.Unmarshal(secret.Data[state.ZarfStateDataKey], &s)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", stateErr, err)
	}
	state.DebugPrint(ctx, s)
	return s, nil
}

// SaveState takes a given state.State and persists it to k8s Cluster secrets.
func (c *Cluster) SaveState(ctx context.Context, s *state.State) error {
	state.DebugPrint(ctx, s)

	data, err := json.Marshal(&s)
	if err != nil {
		return err
	}
	secret := v1ac.Secret(state.ZarfStateSecretName, state.ZarfNamespaceName).
		WithLabels(map[string]string{
			state.ZarfManagedByLabel: "zarf",
		}).
		WithType(corev1.SecretTypeOpaque).
		WithData(map[string][]byte{
			state.ZarfStateDataKey: data,
		})

	_, err = c.Clientset.CoreV1().Secrets(*secret.Namespace).Apply(ctx, secret, metav1.ApplyOptions{Force: true, FieldManager: FieldManagerName})
	if err != nil {
		return fmt.Errorf("unable to apply the zarf state secret: %w", err)
	}
	return nil
}
