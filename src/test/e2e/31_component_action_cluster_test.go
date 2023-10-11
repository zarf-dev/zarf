// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentActionRemove(t *testing.T) {
	t.Log("E2E: Component action remove")
	e2e.SetupWithCluster(t)

	packagePath := filepath.Join("build", fmt.Sprintf("zarf-package-component-actions-%s.tar.zst", e2e.Arch))

	stdOut, stdErr, err := e2e.Zarf("package", "deploy", packagePath, "--confirm", "--components=on-remove")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", packagePath, "--confirm", "--components=on-remove")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "NAME")
	require.Contains(t, stdErr, "DATA")
	require.Contains(t, stdErr, "remove-test-configmap")
	require.Contains(t, stdErr, "Not Found")
}

func TestComponentActionEdgeCases(t *testing.T) {
	t.Log("E2E: Component action edge cases")
	e2e.SetupWithCluster(t)

	sourcePath := filepath.Join("src", "test", "packages", "31-component-actions-edgecases")
	packagePath := fmt.Sprintf("zarf-package-component-actions-edgecases-%s.tar.zst", e2e.Arch)

	stdOut, stdErr, err := e2e.Zarf("package", "create", sourcePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(packagePath)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", packagePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
