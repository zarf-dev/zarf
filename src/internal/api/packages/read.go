// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/go-chi/chi/v5"
)

// Read reads a package from the local filesystem and writes the Zarf.yaml json to the response.
func Read(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Read()")

	path := chi.URLParam(r, "path")

	if pkg, err := utils.ReadPackage(path); err != nil {
		message.ErrorWebf(err, w, "Unable to read the package at: `%s`", path)
	} else {
		common.WriteJSONResponse(w, pkg, http.StatusOK)
	}
}
