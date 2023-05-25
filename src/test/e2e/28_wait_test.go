// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"

	"testing"

	"github.com/stretchr/testify/require"
)

func cleanupNoWaitNS() {
	_, _, _ = e2e.Kubectl("delete", "namespace", "no-wait", "--force=true", "--wait=false", "--grace-period=0")
}

func TestWait(t *testing.T) {
	t.Log("E2E: Helm Wait")
	e2e.SetupWithCluster(t)
	t.Cleanup(cleanupNoWaitNS)

	// Create the package.
	stdOut, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/28-helm-no-wait/", "-o=build", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-helm-no-wait-%s.tar.zst", e2e.Arch)

	deployNoWaitArgs := []string{"package", "deploy", path, "--confirm"}
	stdOut, stdErr, err = e2e.Zarf(deployNoWaitArgs...)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "helm-no-wait", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
