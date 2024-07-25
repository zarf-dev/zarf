// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetries(t *testing.T) {
	t.Log("E2E: Custom Retries")

	tmpDir := t.TempDir()

	buildPath := filepath.Join("src", "test", "packages", "25-evil-dos-games")
	pkgName := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", buildPath, "--tmpdir", tmpDir, "--output", tmpDir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path.Join(tmpDir, pkgName), "--retries", "2", "--timeout", "3s", "--tmpdir", tmpDir, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Retrying in 5s")
	require.Contains(t, e2e.StripMessageFormatting(stdErr), "unable to install chart after 2 attempts")

	_, _, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err)
}
