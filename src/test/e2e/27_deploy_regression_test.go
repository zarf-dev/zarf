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
	// shas for package published 2023-08-08T22:13:51Z
	switch e2e.Arch {
	case "arm64":
		sha = "d4f656981241366a82ef3ed2e175802043a3c5615b72cd819dd94ada27708263"
	case "amd64":
		sha = "6032b1d1029d00932fd44e3a4ac93a5ee62f0732d47b022e821c8688fc6c3c55"
	}

	// Test with command from https://docs.zarf.dev/getting-started/install/
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.2.0@sha256:%s", sha), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
