// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	globalConfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// DeployPackage deploys a package to the Zarf cluster.
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	config := types.PackagerConfig{}
	config.IsInteractive = false

	var body types.APIZarfDeployPayload

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
		return
	}

	if body.InitOpts != nil {
		config.InitOpts = *body.InitOpts
	}
	config.DeployOpts = body.DeployOpts

	globalConfig.CommonOptions.Confirm = true

	pkgClient := packager.NewOrDie(&config)
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Deploy(); err != nil {
		message.ErrorWebf(err, w, "Unable to deploy the zarf package to the cluster")
		return
	}

	common.WriteJSONResponse(w, true, http.StatusCreated)
}

func StreamDeployPackage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	reader, writer, err := os.Pipe()
	if err != nil {
		message.ErrorWebf(err, w, "Error reading stdout: %v", err)
		return
	}
	pterm.SetDefaultOutput(writer)
	pterm.DisableStyling()
	done := r.Context().Done()

	buf := make([]byte, 1024)
	for {
		select {
		case (<-done):
			pterm.SetDefaultOutput(os.Stdout)
			pterm.EnableStyling()
			return
		default:
			n, _ := reader.Read(buf)
			if err != nil {
				message.ErrorWebf(err, w, "Error reading stdout: %v", err)
				return
			}
			if n > 0 {
				fmt.Fprintf(w, "data: %s\n\n", string(buf[:n]))
				w.(http.Flusher).Flush()
			}
		}
	}
}
