// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package components provides api functions for managing zarf components
package components

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
)

// ListDeployingComponents writes a list of packages that have been deployed to the connected cluster.
func ListDeployingComponents(w http.ResponseWriter, _ *http.Request) {
	deployingPackages := config.GetDeployingComponents()
	common.WriteJSONResponse(w, deployingPackages, http.StatusOK)
}
