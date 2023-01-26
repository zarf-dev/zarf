// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentActionRemove(t *testing.T) {
	t.Log("E2E: Component action remove")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	removeArtifacts := []string{
		"test-remove-before.txt",
		"test-remove-after.txt",
	}
	e2e.cleanFiles(removeArtifacts...)
	defer e2e.cleanFiles(removeArtifacts...)

	path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.arch)

	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--components=on-remove")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", path, "--confirm", "--components=on-remove")
	require.NoError(t, err, stdOut, stdErr)

	for _, artifact := range removeArtifacts {
		require.FileExists(t, artifact)
	}
}
