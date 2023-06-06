// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package components provides api functions for managing Zarf components.
package components

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-chi/chi"
)

// ListDeployingComponents writes a list of packages that have been deployed to the connected cluster.
func ListDeployingComponents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	dp, err := cluster.NewClusterOrDie().GetDeployedPackage(name)
	if err != nil {
		message.ErrorWebf(err, w, lang.ErrLoadState)
	}
	common.WriteJSONResponse(w, dp.DeployedComponents, http.StatusOK)
}
