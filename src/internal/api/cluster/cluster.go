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
)

// Summary returns a summary of cluster status.
func Summary(w http.ResponseWriter, _ *http.Request) {
	message.Debug("cluster.Summary()")

	var state types.ZarfState
	var reachable bool
	var distro string
	var hasZarf bool
	var host string

	c, err := cluster.NewClusterWithWait(5 * time.Second)
	reachable = err == nil

	if reachable {
		distro, _ = c.Kube.DetectDistro()
		host = c.Kube.RestConfig.Host
		state, _ = c.LoadZarfState()
		hasZarf = state.Distro != ""
	}

	data := types.ClusterSummary{
		Reachable: reachable,
		HasZarf:   hasZarf,
		Distro:    distro,
		ZarfState: state,
		Host:      host,
	}

	common.WriteJSONResponse(w, data, http.StatusOK)
}
