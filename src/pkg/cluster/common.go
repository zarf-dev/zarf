// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// Cluster is a wrapper for the k8s package that provides Zarf-specific cluster management functions.
type Cluster struct {
	*k8s.K8s
}

const (
	// DefaultTimeout is the default time to wait for a cluster to be ready.
	DefaultTimeout = 30 * time.Second
)

// NewClusterOrDie creates a new Cluster instance and waits up to 30 seconds for the cluster to be ready or throws a fatal error.
func NewClusterOrDie() *Cluster {
	c, err := NewClusterWithWait(DefaultTimeout)
	if err != nil {
		message.Fatalf(err, "Failed to connect to cluster")
	}

	return c
}

// NewClusterWithWait creates a new Cluster instance and waits for the given timeout for the cluster to be ready.
func NewClusterWithWait(timeout time.Duration) (*Cluster, error) {
	spinner := message.NewProgressSpinner("Waiting for cluster connection (%s timeout)", timeout.String())
	defer spinner.Stop()

	c := &Cluster{}
	var err error

	c.K8s, err = k8s.New(message.Debugf)
	if err != nil {
		return nil, err
	}

	err = c.WaitForHealthyCluster(timeout)
	if err != nil {
		return nil, err
	}

	spinner.Success()

	return c, nil
}

// NewCluster creates a new Cluster instance and validates connection to the cluster by fetching the Kubernetes version.
func NewCluster() (*Cluster, error) {
	c := &Cluster{}
	var err error

	c.K8s, err = k8s.New(message.Debugf)
	if err != nil {
		return nil, err
	}

	// Dogsled the version output. We just want to ensure no errors were returned to validate cluster connection.
	_, err = c.GetServerVersion()
	if err != nil {
		return nil, err
	}

	return c, nil
}
