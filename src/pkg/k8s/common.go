// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.	 	
package k8s

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"github.com/go-logr/logr/funcr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewK8sClientWithWait(logger Log, defaultLabels Labels, timeout time.Duration) (*Client, error) {
	k, _ := NewK8sClient(logger, defaultLabels)
	return k, k.WaitForHealthyCluster(timeout)
}

func NewK8sClient(logger Log, defaultLabels Labels) (*Client, error) {
	logger("k8s.NewK8sClient()")

	klog.SetLogger(funcr.New(func(prefix, args string) {
		logger(args)
	}, funcr.Options{}))

	config, clientset, err := connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to k8s cluster: %w", err)
	}

	return &Client{
		RestConfig: config,
		Clientset:  clientset,
		Log:        logger,
		Labels:     defaultLabels,
	}, nil
}

// WaitForHealthyCluster checks for an available K8s cluster every second until timeout.
func (k *Client) WaitForHealthyCluster(timeout time.Duration) error {
	var err error
	var nodes *v1.NodeList
	var pods *v1.PodList
	expired := time.After(timeout)

	for {
		// delay check 1 seconds
		time.Sleep(1 * time.Second)
		select {

		// on timeout abort
		case <-expired:
			return fmt.Errorf("timed out waiting for cluster to report healthy")

		// after delay, try running
		default:
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
			nodes, err = k.GetNodes()
			if err != nil || len(nodes.Items) < 1 {
				k.Log("No nodes reporting healthy yet: %#v\n", err)
				continue
			}

			// Get the cluster pod list
			if pods, err = k.GetAllPods(); err != nil {
				k.Log("Could not get the pod list: %w", err)
				continue
			}

			// Check that at least one pod is in the 'succeeded' or 'running' state
			for _, pod := range pods.Items {
				// If a valid pod is found, return no error
				if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning {
					return nil
				}
			}

			k.Log("No pods reported 'succeeded' or 'running' state yet.")
		}
	}
}

// Use the K8s "client-go" library to get the currently active kube context, in the same way that
// "kubectl" gets it if no extra config flags like "--kubeconfig" are passed
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
