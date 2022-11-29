// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing zarf packages
package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// ListDeployedPackages writes a list of packages that have been deployed to the connected cluster.
func ListDeployedPackages(w http.ResponseWriter, _ *http.Request) {
	c, err := cluster.NewCluster()
	if err != nil {
		message.ErrorWebf(err, w, "Could not connect to cluster")
		return
	}

	deployedPackages, err := c.GetDeployedZarfPackages()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get a list of the deployed Zarf packages")
		return
	}

	common.WriteJSONResponse(w, deployedPackages, http.StatusOK)
}
