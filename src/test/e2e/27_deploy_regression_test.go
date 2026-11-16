// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGHCRDeploy(t *testing.T) {
	t.Log("E2E: GHCR OCI deploy")

	var sha string
	// Digests for dos-games 1.3.0, published January 26, 2026.
	switch e2e.Arch {
	case "arm64":
		sha = "e087e4eeca62f9b6263d69ff567b0046c1881e689925c120f4846a5d7b278c1a"
	case "amd64":
		sha = "a3d5ccd97d617f89f933eb485206945c3eb6836201c3ac97a12d81845724025e"
	}

	// Test with command from https://docs.zarf.dev/getting-started/install/
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.3.0@sha256:%s", sha), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
