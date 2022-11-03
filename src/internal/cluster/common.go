// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

type Cluster struct {
	Kube *k8s.Client
}

const defaultTimeout = 30 * time.Second

var labels = k8s.Labels{
	config.ZarfManagedByLabel: "zarf",
}

// NewClusterOrDie creates a new cluster instance and waits up to 30 seconds for the cluster to be ready or throws a fatal error
func NewClusterOrDie() *Cluster {
	c, err := NewClusterWithWait(defaultTimeout)
	if err != nil {
		message.Fatalf(err, "Failed to connect to cluster")
	}

	return c
}

// NewClusterWithWait creates a new cluster instance and waits for the given timeout for the cluster to be ready
func NewClusterWithWait(timeout time.Duration) (*Cluster, error) {
	c := &Cluster{}
	c.Kube, _ = k8s.NewK8sClient(message.Debugf, labels)
	return c, c.Kube.WaitForHealthyCluster(timeout)
}

// NewCluster creates a new cluster instance without waiting for the cluster to be ready
func NewCluster() (*Cluster, error) {
	c := &Cluster{}
	c.Kube, _ = k8s.NewK8sClient(message.Debugf, labels)
	return c, nil
}
