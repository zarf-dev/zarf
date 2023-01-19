// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"encoding/json"
	"net/http"

	globalConfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/types"
)

// DeployPackage deploys a package to the Zarf cluster.
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	config := types.PackagerConfig{}
	config.IsInteractive = false

	type DeployPayload struct {
		DeployOpts types.ZarfDeployOptions `json:"deployOpts"`
		InitOpts   *types.ZarfInitOptions  `json:"initOpts,omitempty"`
	}

	var body DeployPayload

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
		return
	}

	// Check if init options is empty
	if body.InitOpts != nil {
		config.InitOpts = *body.InitOpts
		config.DeployOpts = body.DeployOpts
	} else {
		config.DeployOpts = body.DeployOpts
	}

	globalConfig.CommonOptions.Confirm = true

	pkgClient := packager.NewOrDie(&config)
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Deploy(); err != nil {
		message.ErrorWebf(err, w, "Unable to deploy the zarf package to the cluster")
		return
	}

	common.WriteJSONResponse(w, true, http.StatusCreated)
}
