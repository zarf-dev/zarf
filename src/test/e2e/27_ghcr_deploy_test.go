// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: GHCR OCI deploy")
	e2e.SetupWithCluster(t)

	// Test with command from https://zarf.dev/install/
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", fmt.Sprintf("oci://ghcr.io/defenseunicorns/packages/dos-games:1.0.0-%s", e2e.Arch), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
