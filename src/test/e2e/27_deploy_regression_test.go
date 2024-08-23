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
		sha = "844dff9aa60345c67b597d5315db5e263cbda01b50643a8d0b7f5ec721f8a16f"
	case "amd64":
		sha = "a44d17160cd6ce7b7b6d4687e7d3f75dad4fedba6670c79665af2e8665a7868e"
	}

	// Test with command from https://docs.zarf.dev/getting-started/install/
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.1.0@sha256:%s", sha), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
