// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	cfg := types.PackagerConfig{}

	var body types.APIZarfDeployPayload

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
		return
	}

	if body.InitOpts != nil {
		cfg.InitOpts = *body.InitOpts
	}
	cfg.PkgOpts = body.DeployOpts

	globalConfig.CommonOptions.Confirm = true

	pkgClient := packager.NewOrDie(&cfg)
	defer pkgClient.ClearTempPaths()

	if err := pkgClient.Deploy(); err != nil {
		message.ErrorWebf(err, w, err.Error())
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
	logStream := io.MultiWriter(message.LogWriter, pw)
	pterm.SetDefaultOutput(logStream)

	scanner := bufio.NewScanner(pr)
	scanner.Split(splitStreamLines)
	done := r.Context().Done()

	// Loop through the scanner and send each line to the stream
	for scanner.Scan() {
		select {
		// If the context is done, reset the output and return
		case (<-done):
			pterm.SetDefaultOutput(message.LogWriter)
			return
		default:
			err := scanner.Err()
			if err != nil {
				message.ErrorWebf(err, w, "Unable to read the stream")
				return
			}
			line := scanner.Text()

			// Clean up the line and send it to the stream
			trimmed := strings.TrimSpace(line)

			fmt.Fprintf(w, "data: %s\n\n", trimmed)
			w.(http.Flusher).Flush()
		}
	}
}

// Splits scanner lines on '\n', '\r', and '\r\n' line endings to ensure the progress and spinner lines show up correctly
func splitStreamLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	// If data ends with '\n', return the line without '\n' or '\r\n'
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// Drop the preceding carriage return if it exists
		if i > 0 && data[i-1] == '\r' {
			return i + 1, data[:i-1], nil
		}

		return i + 1, data[:i], nil
	}
	// if data ends with '\r', return the line without '\r'
	if i := bytes.IndexByte(data, '\r'); i >= 0 {
		return i + 1, data[:i], nil
	}

	// If we're at EOF and we have a final non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}
