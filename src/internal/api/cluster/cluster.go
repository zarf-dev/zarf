// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// Summary returns a summary of cluster status.
func Summary(w http.ResponseWriter, _ *http.Request) {
	message.Debug("cluster.Summary()")

	var state types.ZarfState
	var reachable bool
	var distro string
	var hasZarf bool
	var host string
	var k8sRevision string

	c, err := cluster.NewClusterWithWait(5 * time.Second)
	reachable = err == nil
	if reachable {
		distro, _ = c.Kube.DetectDistro()
		host = c.Kube.RestConfig.Host
		state, _ = c.LoadZarfState()
		hasZarf = state.Distro != ""
		k8sRevision = getServerVersion(c.Kube)
	}

	data := types.ClusterSummary{
		Reachable:   reachable,
		HasZarf:     hasZarf,
		Distro:      distro,
		ZarfState:   state,
		Host:        host,
		K8sRevision: k8sRevision,
	}

	common.WriteJSONResponse(w, data, http.StatusOK)
}

func getServerVersion(k *k8s.K8s) string {
	info, _ := k.Clientset.DiscoveryClient.ServerVersion()

	return info.String()
}

func getClusterName(k *k8s.K8s) string {

	// Get all nodes
	nodes, err := k.GetNodes()
	// if no nodes are found just return empty string.
	if err != nil {
		return ""
	}

	// Grab first node the kubernetes.io/hostname (control-plane) will be same for all nodes.
	node := nodes.Items[0]

	labels := node.GetLabels()

	name, _ := labels["kubernetes.io/hostname"]

	return name
}
