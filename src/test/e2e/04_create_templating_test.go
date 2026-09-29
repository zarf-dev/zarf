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

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestCreateTemplating(t *testing.T) {
	t.Log("E2E: Create Templating")

	outPath := t.TempDir()
	templatingPath := filepath.Join(outPath, fmt.Sprintf("zarf-package-templating-%s.tar.zst", e2e.Arch))
	fileFoldersPath := filepath.Join(outPath, fmt.Sprintf("zarf-package-file-folders-templating-sbom-%s.tar.zst", e2e.Arch))

	// Test that not specifying a package variable results in an error
	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "-o", outPath, "--confirm")
	require.Error(t, err)

	// Test a simple package variable example with `--set` (will fail to pull an image if this is not set correctly)
	_, _, err = e2e.Zarf(t, "package", "create", "src/test/packages/04-templating", "-o", outPath, "--set", "PODINFO_VERSION=6.4.0", "--confirm")
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(context.Background(), templatingPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	expectedConstant := api.Constant{Name: "PODINFO_VERSION", Value: "6.4.0", Pattern: "^[\\w\\-\\.]+$"}
	require.Contains(t, pkgLayout.Definition().Constants, expectedConstant)

	// Test templating files and folders.
	_, _, err = e2e.Zarf(t, "package", "create", "src/test/packages/04-file-folders-templating-sbom/", "-o", outPath, "--confirm")
	require.NoError(t, err)

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
