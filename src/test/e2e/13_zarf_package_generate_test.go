// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestZarfDevGenerate(t *testing.T) {
	t.Log("E2E: Zarf Dev Generate")

	t.Run("Test arguments and flags", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf("dev", "generate")
		require.Error(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf("dev", "generate", "podinfo")
		require.Error(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf("dev", "generate", "podinfo", "--url", "https://zarf.dev")
		require.Error(t, err, stdOut, stdErr)
	})

	t.Run("Test generate podinfo", func(t *testing.T) {
		tmpDir := t.TempDir()

		url := "https://github.com/stefanprodan/podinfo.git"
		version := "6.4.0"
		gitPath := "charts/podinfo"


		stdOut, stdErr, err := e2e.Zarf("dev", "generate", "podinfo", "--url", url, "--version", version, "--gitPath", gitPath, "--output-directory", tmpDir)
		require.NoError(t, err, stdOut, stdErr)

		zarfPackage := types.ZarfPackage{}
		err = utils.ReadYaml(tmpDir+"/zarf.yaml", &zarfPackage)
		require.NoError(t, err)
		require.Equal(t, zarfPackage.Components[0].Charts[0].URL, url)
		require.Equal(t, zarfPackage.Components[0].Charts[0].Version, version)
		require.Equal(t, zarfPackage.Components[0].Charts[0].GitPath, gitPath)

	})
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
