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
)

func TestCreateTemplating(t *testing.T) {
	t.Log("E2E: Create Templating")

	// run `zarf package create` with a specified image cache location
	tmpdir := t.TempDir()
	decompressPath := filepath.Join(tmpdir, ".package-decompressed")
	sbomPath := filepath.Join(tmpdir, ".sbom-location")

	pkgName := fmt.Sprintf("zarf-package-templating-%s.tar.zst", e2e.Arch)

	// Test that not specifying a package variable results in an error
	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "--confirm")
	require.Error(t, err)

	// Test a simple package variable example with `--set` (will fail to pull an image if this is not set correctly)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "--set", "PODINFO_VERSION=6.4.0", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "t", "archiver", "decompress", pkgName, decompressPath, "--unarchive-all")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the constant in the zarf.yaml is replaced correctly
	builtConfig, err := os.ReadFile(decompressPath + "/zarf.yaml")
	require.NoError(t, err)
	require.Contains(t, string(builtConfig), "name: PODINFO_VERSION\n  value: 6.4.0")

	// Test that files and file folders template and handle SBOMs correctly
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "src/test/packages/04-file-folders-templating-sbom/", "--sbom-out", sbomPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Creating SBOMs for 0 images and 2 components with files.")

	fileFoldersPkgName := fmt.Sprintf("zarf-package-file-folders-templating-sbom-%s.tar.zst", e2e.Arch)

	// Deploy the package and look for the variables in the output
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", fileFoldersPkgName, "--set", "DOGGO=doggy", "--set", "KITTEH=meowza", "--set", "PANDA=pandemonium", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "A doggy barks!")
	require.Contains(t, stdErr, "  - meowza")
	require.Contains(t, stdErr, "# Total pandemonium")

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

	e2e.CleanFiles(pkgName, fileFoldersPkgName)
}
