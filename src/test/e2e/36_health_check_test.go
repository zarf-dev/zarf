// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"

	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthChecks(t *testing.T) {
	t.Log("E2E: Health Checks")

	_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/36-health-checks", "-o=build", "--confirm")
	require.NoError(t, err)

	path := fmt.Sprintf("build/zarf-package-health-checks-%s.tar.zst", e2e.Arch)

	_, _, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err)

	defer func() {
		_, _, err = e2e.Zarf(t, "package", "remove", "health-checks", "--confirm")
		require.NoError(t, err)
	}()

	stdOut, _, err := e2e.Kubectl(t, "get", "pod", "ready-pod", "-n", "health-checks", "-o", "jsonpath={.status.phase}")
	require.NoError(t, err)
	require.Equal(t, "Running", stdOut)
}
