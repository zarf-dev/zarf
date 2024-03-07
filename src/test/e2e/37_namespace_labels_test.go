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

func TestNamespaceLabels(t *testing.T) {
	namespace := "ns-labels-dos-games"
	defer e2e.Kubectl("delete", "namespace", namespace)

	t.Log("E2E: Namespace Labels")
	e2e.SetupWithCluster(t)
	buildPath := filepath.Join("src", "test", "packages", "37-namespace-labels")
	packageName := fmt.Sprintf("zarf-package-namespace-labels-%s.tar.zst", e2e.Arch)

	stdOut, stdErr, err := e2e.Zarf("package", "create", buildPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(packageName)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", packageName, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl("get", "namespace", namespace, "--show-labels")
	t.Log(stdOut)
	require.Contains(t, stdOut, "app.kubernetes.io/managed-by=zarf")
	require.Contains(t, stdOut, "gamerNamespace=dos")
}
