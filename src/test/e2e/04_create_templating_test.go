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

	e2e.setup(t)
	defer e2e.teardown(t)

	// run `zarf package create` with a specified image cache location
	cachePath := filepath.Join(os.TempDir(), ".cache-location")
	decompressPath := filepath.Join(os.TempDir(), ".package-decompressed")

	e2e.cleanFiles(cachePath, decompressPath)

	pkgName := fmt.Sprintf("zarf-package-variables-%s.tar.zst", e2e.arch)

	// Test that not specifying a package variable results in an error
	_, stdErr, _ := e2e.execZarfCommand("package", "create", "examples/variables", "--confirm", "--zarf-cache", cachePath)
	expectedOutString := "variable 'NGINX_VERSION' must be '--set' when using the '--confirm' flag"
	require.Contains(t, stdErr, "", expectedOutString)

	// Test a simple package variable example with `--set` (will fail to pull an image if this is not set correctly)
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "examples/variables", "--set", "NGINX_VERSION=1.23.3", "--confirm", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("t", "archiver", "decompress", pkgName, decompressPath, "--decompress-all", "-l=trace")
	require.NoError(t, err, stdOut, stdErr)

	// Check that the constant in the zarf.yaml is replaced correctly
	builtConfig, err := os.ReadFile(decompressPath + "/zarf.yaml")
	require.NoError(t, err)
	require.Contains(t, string(builtConfig), "name: NGINX_VERSION\n  value: 1.23.3")

	e2e.cleanFiles(cachePath, decompressPath, pkgName)
}
