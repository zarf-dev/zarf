// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
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
	packageTemplatePath := filepath.Join("src", "test", "packages", "15-component-publish-v1beta1", "zarf.tpl.yaml")
	packageTemplate, err := os.ReadFile(packageTemplatePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.tpl.yaml"), packageTemplate, 0o600))
	stdOut, stdErr, err = e2e.ZarfInDir(t, packageDir, "dev", "template", "--set", "registryURL="+registryURL)
	require.NoError(t, err, stdOut, stdErr)

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

// FIXME: this should probably be a unit test
func TestComponentPublishArchitectureIndexAndFlavorReference(t *testing.T) {
	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	componentName := "indexed-component"
	flavor := "hardened"
	otherArchitecture := "amd64"
	if e2e.Arch == otherArchitecture {
		otherArchitecture = "arm64"
	}

	writeVariant := func(t *testing.T, architecture string) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "variant.txt"), []byte(architecture), 0o600))
		componentYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: %s
  version: 0.0.1
  flavor: %s
  architecture: %s
component:
  files:
    - source: variant.txt
      destination: /tmp/variant.txt
`, componentName, flavor, architecture)
		componentPath := filepath.Join(dir, "component.yaml")
		require.NoError(t, os.WriteFile(componentPath, []byte(componentYAML), 0o600))
		return componentPath
	}

	for _, architecture := range []string{otherArchitecture, e2e.Arch} {
		stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", writeVariant(t, architecture), "oci://"+registryURL, "--plain-http", "--confirm")
		require.NoError(t, err, stdOut, stdErr)
	}

	packageDir := t.TempDir()
	packageYAML := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: indexed-component-import
  architecture: %s
components:
  - name: imported-variant
    import:
      remote:
        - url: oci://%s/%s:0.0.1-%s
`, e2e.Arch, registryURL, componentName, flavor)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.yaml"), []byte(packageYAML), 0o600))

	packageOutput := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packageDir, "-o", packageOutput, "--plain-http", "--skip-sbom", "--flavor", flavor, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packagePath := filepath.Join(packageOutput, fmt.Sprintf("zarf-package-indexed-component-import-%s-%s.tar.zst", e2e.Arch, flavor))
	pkgLayout, err := layout.LoadFromTar(t.Context(), packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	componentExtractDir := t.TempDir()
	filesDir, err := pkgLayout.GetComponentDir(t.Context(), componentExtractDir, "imported-variant", layout.FilesComponentDir)
	require.NoError(t, err)
	contents, err := os.ReadFile(filepath.Join(filesDir, "0", "variant.txt"))
	require.NoError(t, err)
	require.Equal(t, e2e.Arch, string(contents))
}
