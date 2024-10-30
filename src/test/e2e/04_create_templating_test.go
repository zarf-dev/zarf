// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

func TestCreateTemplating(t *testing.T) {
	t.Log("E2E: Create Templating")

	sbomPath := t.TempDir()
	outPath := t.TempDir()
	templatingPath := filepath.Join(outPath, fmt.Sprintf("zarf-package-templating-%s.tar.zst", e2e.Arch))
	fileFoldersPath := filepath.Join(outPath, fmt.Sprintf("zarf-package-file-folders-templating-sbom-%s.tar.zst", e2e.Arch))

	// Test that not specifying a package variable results in an error
	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "-o", outPath, "--confirm")
	require.Error(t, err)

	// Test a simple package variable example with `--set` (will fail to pull an image if this is not set correctly)
	_, _, err = e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "-o", outPath, "--set", "PODINFO_VERSION=6.4.0", "--confirm")
	require.NoError(t, err)

	pkgLayout, err := layout2.LoadFromTar(context.Background(), templatingPath, layout2.PackageLayoutOptions{})
	require.NoError(t, err)
	expectedConstant := v1alpha1.Constant{Name: "PODINFO_VERSION", Value: "6.4.0", Pattern: "^[\\w\\-\\.]+$"}
	require.Contains(t, pkgLayout.Pkg.Constants, expectedConstant)

	// Test that files and file folders template and handle SBOMs correctly
	_, _, err = e2e.Zarf(t, "package", "create", "src/test/packages/04-file-folders-templating-sbom/", "-o", outPath, "--sbom-out", sbomPath, "--confirm")
	require.NoError(t, err)

	// Ensure that the `requirements.txt` files are discovered correctly
	require.FileExists(t, filepath.Join(sbomPath, "file-folders-templating-sbom", "compare.html"))
	require.FileExists(t, filepath.Join(sbomPath, "file-folders-templating-sbom", "sbom-viewer-zarf-component-folders.html"))
	foldersJSON, err := os.ReadFile(filepath.Join(sbomPath, "file-folders-templating-sbom", "zarf-component-folders.json"))
	require.NoError(t, err)
	require.Contains(t, string(foldersJSON), "numpy")
	_, err = os.ReadFile(filepath.Join(sbomPath, "file-folders-templating-sbom", "sbom-viewer-zarf-component-files.html"))
	require.NoError(t, err)
	filesJSON, err := os.ReadFile(filepath.Join(sbomPath, "file-folders-templating-sbom", "zarf-component-files.json"))
	require.NoError(t, err)
	require.Contains(t, string(filesJSON), "pandas")

	// Deploy the package and look for the variables in the output
	workingPath := t.TempDir()
	_, _, err = e2e.ZarfInDir(t, workingPath, "package", "deploy", fileFoldersPath, "--set", "DOGGO=doggy", "--set", "KITTEH=meowza", "--set", "PANDA=pandemonium", "--confirm")
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(workingPath, "temp", "requirements.txt"))
	require.NoError(t, err)
	require.Equal(t, "# Total pandemonium\npandas==1.5.0\n", string(b))

	b, err = os.ReadFile(filepath.Join(workingPath, "temp", "include-files", "simple.txt"))
	require.NoError(t, err)
	require.Equal(t, "A doggy barks!\n", string(b))

	b, err = os.ReadFile(filepath.Join(workingPath, "temp", "include-files", "something.yaml"))
	require.NoError(t, err)
	require.Equal(t, "something:\n  - a\n  - meowza\n  - meows\n", string(b))
}
