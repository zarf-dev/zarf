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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/kstatus/watcher"

	pkgkubernetes "github.com/defenseunicorns/pkg/kubernetes"

	"github.com/zarf-dev/zarf/src/pkg/message"
)

const (
	// DefaultTimeout is the default time to wait for a cluster to be ready.
	DefaultTimeout = 30 * time.Second
	// AgentLabel is used to give instructions to the Zarf agent
	AgentLabel = "zarf.dev/agent"
)

// Cluster Zarf specific cluster management functions.
type Cluster struct {
	Clientset  kubernetes.Interface
	RestConfig *rest.Config
	Watcher    watcher.StatusWatcher
}

// NewClusterWithWait creates a new Cluster instance and waits for the given timeout for the cluster to be ready.
func NewClusterWithWait(ctx context.Context) (*Cluster, error) {
	spinner := message.NewProgressSpinner("Waiting for cluster connection")
	defer spinner.Stop()

	c, err := NewCluster()
	if err != nil {
		return nil, err
	}
	// returning false, or an error continues polling, true stops it
	err = wait.PollUntilContextCancel(ctx, time.Second, false, func(context.Context) (bool, error) {
		nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if len(nodeList.Items) < 1 {
			return false, fmt.Errorf("cluster does not have any nodes")
		}
		pods, err := c.Clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodRunning {
				return true, nil
			}
		}
		return false, fmt.Errorf("no pods are in succeeded or running state")
	})
	if err != nil {
		return nil, err
	}

	spinner.Success()

	return c, nil
}

// NewCluster creates a new Cluster instance and validates connection to the cluster by fetching the Kubernetes version.
func NewCluster() (*Cluster, error) {
	clusterErr := errors.New("unable to connect to the cluster")
	clientset, config, err := pkgkubernetes.ClientAndConfig()
	if err != nil {
		return nil, errors.Join(clusterErr, err)
	}
	watcher, err := pkgkubernetes.WatcherForConfig(config)
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
