// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/go-logr/logr/funcr"
	"k8s.io/client-go/kubernetes"

	// Include the cloud auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// cannot import config.ZarfManagedByLabel due to import cycle
const zarfManagedByLabel = "app.kubernetes.io/managed-by"

// New creates a new K8s client.
func New(logger Log, defaultLabels Labels) (*K8s, error) {
	klog.SetLogger(funcr.New(func(_, args string) {
		logger(args)
	}, funcr.Options{}))

	config, clientset, err := connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to k8s cluster: %w", err)
	}

	return &K8s{
		RestConfig: config,
		Clientset:  clientset,
		Log:        logger,
		Labels:     defaultLabels,
	}, nil
}

// WaitForHealthyCluster checks for an available K8s cluster every second until timeout.
func (k *K8s) WaitForHealthyCluster(ctx context.Context) error {
	var err error
	var nodes *v1.NodeList
	var pods *v1.PodList

	for {
		if k.RestConfig == nil || k.Clientset == nil {
			config, clientset, err := connect()
			if err != nil {
				k.Log("Cluster connection not available yet: %w", err)
				continue
			}

			k.RestConfig = config
			k.Clientset = clientset
		}

		// Make sure there is at least one running Node
		nodes, err = k.GetNodes(ctx)
		if err != nil || len(nodes.Items) < 1 {
			k.Log("No nodes reporting healthy yet: %#v\n", err)
			continue
		}

		// Get the cluster pod list
		if pods, err = k.GetAllPods(ctx); err != nil {
			k.Log("Could not get the pod list: %w", err)
			continue
		}

		// Check that at least one pod is in the 'succeeded' or 'running' state
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning {
				return nil
			}
		}

		k.Log("No pods reported 'succeeded' or 'running' state yet.")

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for cluster to report healthy: %w", ctx.Err())
		case <-time.After(1 * time.Second):
		}
	}
}

// Use the K8s "client-go" library to get the currently active kube context, in the same way that
// "kubectl" gets it if no extra config flags like "--kubeconfig" are passed.
func connect() (config *rest.Config, clientset *kubernetes.Clientset, err error) {
	// Build the config from the currently active kube context in the default way that the k8s client-go gets it, which
	// is to look at the KUBECONFIG env var
	config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{}).ClientConfig()

	if err != nil {
		return nil, nil, err
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return config, clientset, nil
}
