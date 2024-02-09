// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZarfDevGenerate(t *testing.T) {
	t.Log("E2E: Zarf Dev Generate")

	stdOut, stdErr, err := e2e.Zarf("dev", "generate", "podinfo", "--url", "https://github.com/stefanprodan/podinfo.git", "--version", "6.4.0", "--gitPath", "charts/podinfo")
	require.NoError(t, err, stdOut, stdErr)



	// Assert the ZarfPackageConfig
	// ```yaml
	// kind: ZarfPackageConfig
	// metadata:
	// name: podinfo # This is <package-name>

	// components:
	// - name: podinfo # This is <package-name>
	// 	required: true
	// 	charts:
	// 	- name: podinfo # This is <chart-name> from the Chart.yaml
	// 		version: 6.4.0 # This is --version
	// 		namespace: podinfo # This is <chart-name> from the Chart.yaml
	// 		url: https://github.com/stefanprodan/podinfo.git # This is --url
	// 		gitPath: charts/podinfo # This is --gitPath
	// 	images: # These are autodiscovered with find-images logic
	// 	- ghcr.io/stefanprodan/podinfo:6.4.0
	// 	# This is the cosign signature for the podinfo image for image signature verification
	// 	- ghcr.io/stefanprodan/podinfo:sha256-57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8.sig
	// ```
}
