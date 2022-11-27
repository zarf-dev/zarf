// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cluster contains zarf-specific cluster management functions
package cluster

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// Read the zarf state secret from the cluster, if it exists.
func ReadState(w http.ResponseWriter, r *http.Request) {
	message.Debug("state.Read()")

	data, err := cluster.NewClusterOrDie().LoadZarfState()
	if err != nil {
		message.ErrorWebf(err, w, lang.ErrLoadState)
	}

	if data.Distro == "" {
		common.WriteEmpty(w)
	} else {
		common.WriteJSONResponse(w, data, http.StatusOK)
	}
}

// Update the zarf state secret in the cluster.
func UpdateState(w http.ResponseWriter, r *http.Request) {
	message.Debug("state.Update()")

	var data types.ZarfState

	if err := cluster.NewClusterOrDie().SaveZarfState(data); err != nil {
		message.ErrorWebf(err, w, lang.ErrLoadState)
	} else {
		common.WriteJSONResponse(w, data, http.StatusCreated)
	}
}
