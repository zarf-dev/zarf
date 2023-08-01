// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains Zarf-specific cluster management functions.
package cluster

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"k8s.io/client-go/tools/clientcmd"
)

// Summary returns a summary of cluster status.
func Summary(w http.ResponseWriter, _ *http.Request) {
	message.Debug("cluster.Summary()")

	var state *types.ZarfState
	var reachable bool
	var distro string
	var hasZarf bool
	var k8sRevision string

	c, err := cluster.NewClusterWithWait(5*time.Second, false)
	rawConfig, _ := clientcmd.NewDefaultClientConfigLoadingRules().GetStartingConfig()

	reachable = err == nil
	if reachable {
		distro, _ = c.DetectDistro()
		state, _ = c.LoadZarfState()
		hasZarf = state != nil
		k8sRevision = getServerVersion(c)
	}

	data := types.ClusterSummary{
		Reachable:   reachable,
		HasZarf:     hasZarf,
		Distro:      distro,
		ZarfState:   state,
		K8sRevision: k8sRevision,
		RawConfig:   rawConfig,
	}

	common.WriteJSONResponse(w, data, http.StatusOK)
}

// Retrieve and return the k8s revision.
func getServerVersion(c *cluster.Cluster) string {
	info, _ := c.Clientset.DiscoveryClient.ServerVersion()

	return info.String()
}
