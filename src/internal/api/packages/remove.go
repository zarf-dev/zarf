// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
)

// RemovePackage removes a package that has been deployed to the cluster.
func RemovePackage(w http.ResponseWriter, r *http.Request) {
	// Get the components to remove from the (optional) query params
	components := r.URL.Query().Get("components")

	// Get the name of the package we're removing from the URL params
	name := chi.URLParam(r, "name")

	// Setup the packager
	pkg, err := packager.New(&types.PackagerConfig{
		DeployOpts: types.ZarfDeployOptions{
			Components: components,
		},
	})
	defer pkg.ClearTempPaths()

	if err != nil {
		message.ErrorWebf(err, w, "Unable to remove the zarf package from the cluster")
	}

	// Remove the package
	if err := pkg.Remove(name); err != nil {
		message.ErrorWebf(err, w, "Unable to remove the zarf package from the cluster")
		return
	}

	common.WriteJSONResponse(w, nil, http.StatusOK)
}
