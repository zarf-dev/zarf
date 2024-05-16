// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// Cluster is a wrapper for the k8s package that provides Zarf-specific cluster management functions.
type Cluster struct {
	Clientset  kubernetes.Interface
	restConfig *rest.Config
}

const (
	// DefaultTimeout is the default time to wait for a cluster to be ready.
	DefaultTimeout = 30 * time.Second
	agentLabel     = "zarf.dev/agent"
)

// NewClusterWithWait creates a new Cluster instance and waits for the given timeout for the cluster to be ready.
func NewClusterWithWait(ctx context.Context) (*Cluster, error) {
	spinner := message.NewProgressSpinner("Waiting for cluster connection")
	defer spinner.Stop()

	restConfig, client, err := kubernetesClients()
	if err != nil {
		return nil, err
	}
	err = waitForHealthyCluster(ctx, client)
	if err != nil {
		return nil, err
	}
	spinner.Success()
	return &Cluster{
		Clientset:  client,
		restConfig: restConfig,
	}, nil
}

// NewCluster creates a new Cluster instance and validates connection to the cluster by fetching the Kubernetes version.
func NewCluster() (*Cluster, error) {
	restConfig, client, err := kubernetesClients()
	if err != nil {
		return nil, err
	}
	_, err = client.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to get Kubernetes version from the cluster : %w", err)
	}
	return &Cluster{
		Clientset:  client,
		restConfig: restConfig,
	}, nil
}

func kubernetesClients() (config *rest.Config, clientset *kubernetes.Clientset, err error) {
	// Build the config from the currently active kube context in the default way that the k8s client-go gets it, which
	// is to look at the KUBECONFIG env var
	config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), nil).ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return config, clientset, nil
}

func waitForHealthyCluster(ctx context.Context, client kubernetes.Interface) error {
	const waitDuration = 1 * time.Second
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("error waiting for cluster to report healthy: %w", ctx.Err())
		case <-timer.C:
			// Make sure there is at least one running Node
			nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil || len(nodeList.Items) < 1 {
				message.Debugf("No nodes reporting healthy yet: %v\n", err)
				timer.Reset(waitDuration)
				continue
			}

			// Check that at least one pod is in the succeeded or running state
			podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
			if err != nil {
				message.Debugf("Could not get the pod list: %w", err)
				timer.Reset(waitDuration)
				continue
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodRunning {
					return nil
				}
			}

			message.Debugf("No pods reported 'succeeded' or 'running' state yet.")
			timer.Reset(waitDuration)
		}
	}
}

// NewZarfManagedNamespace returns a corev1.Namespace with Zarf-managed labels
// TODO: Move to a better place
func (c *Cluster) NewZarfManagedNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				config.ZarfManagedByLabel: "zarf",
			},
		},
	}
}

// TODO: Find a better place for this code
// TODO: This function needs a complete refactor

// PodLookup is a struct for specifying a pod to target for data injection or lookups.
type PodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}

// PodFilter is a function that returns true if the pod should be targeted for data injection or lookups.
type PodFilter func(pod corev1.Pod) bool

type GeneratedPKI struct {
	CA   []byte `json:"ca"`
	Cert []byte `json:"cert"`
	Key  []byte `json:"key"`
}

// WaitForPodsAndContainers attempts to find pods matching the given selector and optional inclusion filter
// It will wait up to 90 seconds for the pods to be found and will return a list of matching pod names
// If the timeout is reached, an empty list will be returned.
func (c *Cluster) WaitForPodsAndContainers(ctx context.Context, target PodLookup, include PodFilter) []corev1.Pod {
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-waitCtx.Done():
			message.Debugf("Pod lookup failed: %v", ctx.Err())
			return nil
		case <-timer.C:
			podList, err := c.Clientset.CoreV1().Pods(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: target.Selector})
			if err != nil {
				message.Debugf("Unable to find matching pods: %w", err)
				return nil
			}

			message.Debugf("Found %d pods for target %#v", len(podList.Items), target)

			var readyPods = []corev1.Pod{}

			// Sort the pods from newest to oldest
			sort.Slice(podList.Items, func(i, j int) bool {
				return podList.Items[i].CreationTimestamp.After(podList.Items[j].CreationTimestamp.Time)
			})

			for _, pod := range podList.Items {
				message.Debugf("Testing pod %q", pod.Name)

				// If an include function is provided, only keep pods that return true
				if include != nil && !include(pod) {
					continue
				}

				// Handle container targeting
				if target.Container != "" {
					message.Debugf("Testing pod %q for container %q", pod.Name, target.Container)

					// Check the status of initContainers for a running match
					for _, initContainer := range pod.Status.InitContainerStatuses {
						isRunning := initContainer.State.Running != nil
						if initContainer.Name == target.Container && isRunning {
							// On running match in initContainer break this loop
							readyPods = append(readyPods, pod)
							break
						}
					}

					// Check the status of regular containers for a running match
					for _, container := range pod.Status.ContainerStatuses {
						isRunning := container.State.Running != nil
						if container.Name == target.Container && isRunning {
							readyPods = append(readyPods, pod)
							break
						}
					}
				} else {
					status := pod.Status.Phase
					message.Debugf("Testing pod %q phase, want (%q) got (%q)", pod.Name, corev1.PodRunning, status)
					// Regular status checking without a container
					if status == corev1.PodRunning {
						readyPods = append(readyPods, pod)
						break
					}
				}
			}
			if len(readyPods) > 0 {
				return readyPods
			}
			timer.Reset(3 * time.Second)
		}
	}
}
