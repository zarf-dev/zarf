// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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

// StreamDeployPackage Establishes a stream that redirects pterm output to the stream
// Resets the output to std.err after the stream connection is closed
func StreamDeployPackage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	pr, pw, _ := os.Pipe()

	pterm.SetDefaultOutput(pw)
	pterm.DisableStyling()

	scanner := bufio.NewScanner(pr)
	done := r.Context().Done()

	for {
		select {
		case (<-done):
			pterm.SetDefaultOutput(os.Stderr)
			pterm.EnableStyling()
			return
		default:
			n := scanner.Scan()
			if err := scanner.Err(); err != nil {
				message.ErrorWebf(err, w, "Error reading stdout: %v", err)
				return
			}
			if n {
				// TODO: dig in to alternatives to trim
				// Some output is not sent unless trimmed
				// Specifically the output from the loading spinner.
				trimmed := strings.TrimSpace(scanner.Text())
				fmt.Fprintf(w, "data: %s\n\n", trimmed)
				w.(http.Flusher).Flush()
			}
		}
	}
}
