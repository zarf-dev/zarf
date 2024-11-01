// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"

	"github.com/avast/retry-go/v4"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
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
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
	Watcher    watcher.StatusWatcher
}

// NewClusterWithWait creates a new Cluster instance and waits for the given timeout for the cluster to be ready.
func NewClusterWithWait(ctx context.Context) (*Cluster, error) {
	start := time.Now()
	l := logger.From(ctx)
	spinner := message.NewProgressSpinner("Waiting for cluster connection")
	defer spinner.Stop()
	l.Info("waiting for cluster connection")

	c, err := NewCluster()
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

	spinner.Success()
	l.Debug("done waiting for cluster, connected", "duration", time.Since(start))

	return c, nil
}

// NewCluster creates a new Cluster instance and validates connection to the cluster by fetching the Kubernetes version.
func NewCluster() (*Cluster, error) {
	clusterErr := errors.New("unable to connect to the cluster")
	clientset, config, err := ClientAndConfig()
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	watcher, err := WatcherForConfig(config)
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	c := &Cluster{
		Clientset:  clientset,
		RestConfig: config,
		Watcher:    watcher,
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
