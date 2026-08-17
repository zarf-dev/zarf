// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestComponentPublishRemoteImport(t *testing.T) {
	componentPath := filepath.Join("src", "test", "packages", "15-component-publish-v1beta1", "component.yaml")

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packageDir := t.TempDir()
	packageConfigPath := filepath.Join("src", "test", "packages", "15-component-publish-v1beta1", layout.ZarfYAML)
	packageConfig, err := os.ReadFile(packageConfigPath)
	require.NoError(t, err)
	packageConfig = []byte(strings.ReplaceAll(string(packageConfig), "REGISTRY_URL", registryURL))
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), packageConfig, 0o600))

	packageOutput := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", packageOutput, "--plain-http", "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packagePath := filepath.Join(packageOutput, fmt.Sprintf("zarf-package-component-remote-import-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(t.Context(), packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))

	componentExtractDir := t.TempDir()
	chartsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ChartsComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(chartsDir, "local-chart.tgz"))

	valuesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ValuesComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(valuesDir, "local-chart-0"))

	filesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.FilesComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(filesDir, "0", "local-file.txt"))

	manifestsDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-component", layout.ManifestsComponentDir)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(manifestsDir, "local-manifest-0.yaml"))
	require.FileExists(t, filepath.Join(manifestsDir, "kustomization-local-kustomization-0.yaml"))
}
