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

func TestCreateCache(t *testing.T) {
	t.Log("E2E: Create Cache")

	e2e.setup(t)
	defer e2e.teardown(t)

	// run `zarf package create` with a specified image cache location
	cachePath := filepath.Join(os.TempDir(), ".cache-location")

	e2e.cleanFiles(cachePath)
	// defer the cleanFiles action because of how the zarf command is launched as a separate process
	// and may return earlier clearing the cache and not properly checking for a failure
	defer e2e.cleanFiles(cachePath)

	pkgName := fmt.Sprintf("zarf-package-git-data-%s-v1.0.0.tar.zst", e2e.arch)

	// Test that not specifying a package variable results in an error
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "examples/git-data", "--confirm", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	// Test that the cache can be used at least once
	stdOut, stdErr, err = e2e.execZarfCommand("package", "create", "examples/git-data", "--confirm", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	// Test that the cache is not corrupted when used
	stdOut, stdErr, err = e2e.execZarfCommand("package", "create", "examples/git-data", "--confirm", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	e2e.cleanFiles(pkgName)
}
