// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

func TestZarfDevGenerate(t *testing.T) {
	t.Log("E2E: Zarf Dev Generate")

	t.Run("Test generate podinfo", func(t *testing.T) {
		tmpDir := t.TempDir()

		url := "https://github.com/stefanprodan/podinfo.git"
		version := "6.4.0"
		gitPath := "charts/podinfo"

		stdOut, stdErr, err := e2e.Zarf("dev", "generate", "podinfo", "--url", url, "--version", version, "--gitPath", gitPath, "--output-directory", tmpDir)
		require.NoError(t, err, stdOut, stdErr)

		zarfPackage := types.ZarfPackage{}
		packageLocation := filepath.Join(tmpDir, layout.ZarfYAML)
		err = utils.ReadYaml(packageLocation, &zarfPackage)
		require.NoError(t, err)
		require.Equal(t, zarfPackage.Components[0].Charts[0].URL, url)
		require.Equal(t, zarfPackage.Components[0].Charts[0].Version, version)
		require.Equal(t, zarfPackage.Components[0].Charts[0].GitPath, gitPath)
		require.NotEmpty(t, zarfPackage.Components[0].Images)
	})
}
