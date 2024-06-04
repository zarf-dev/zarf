// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	// Include the cloud auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	pkgkubernetes "github.com/defenseunicorns/pkg/kubernetes"
)

const (
	// ZarfManagedByLabel is used to denote Zarf manages the lifecycle of a resource
	ZarfManagedByLabel = "app.kubernetes.io/managed-by"
	// AgentLabel is used to give instructions to the Zarf agent
	AgentLabel = "zarf.dev/agent"
)

// New creates a new K8s client.
func New(logger Log) (*K8s, error) {
	klog.SetLogger(funcr.New(func(_, args string) {
		logger(args)
	}, funcr.Options{}))

	config, clientset, err := connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to k8s cluster: %w", err)
	}

	watcher, err := pkgkubernetes.WatcherForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8s{
		RestConfig: config,
		Clientset:  clientset,
		Watcher:    watcher,
		Log:        logger,
	}, nil
}

// WaitForHealthyCluster checks for an available K8s cluster every second until timeout.
func (k *K8s) WaitForHealthyCluster(ctx context.Context) error {
	const waitDuration = 1 * time.Second

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("error waiting for cluster to report healthy: %w", ctx.Err())
		case <-timer.C:
			if k.RestConfig == nil || k.Clientset == nil {
				config, clientset, err := connect()
				if err != nil {
					k.Log("Cluster connection not available yet: %w", err)
					timer.Reset(waitDuration)
					continue
				}

				k.RestConfig = config
				k.Clientset = clientset
			}

			// Make sure there is at least one running Node
			nodeList, err := k.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil || len(nodeList.Items) < 1 {
				k.Log("No nodes reporting healthy yet: %v\n", err)
				timer.Reset(waitDuration)
				continue
			}

			// Get the cluster pod list
			pods, err := k.GetAllPods(ctx)
			if err != nil {
				k.Log("Could not get the pod list: %w", err)
				timer.Reset(waitDuration)
				continue
			}

			// Check that at least one pod is in the 'succeeded' or 'running' state
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodRunning {
					return nil
				}
			}

			k.Log("No pods reported 'succeeded' or 'running' state yet.")
			timer.Reset(waitDuration)
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
