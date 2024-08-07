// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

func TestGHCRDeploy(t *testing.T) {
	t.Log("E2E: GHCR OCI deploy")

	var sha string
	switch e2e.Arch {
	case "arm64":
		sha = "af7033ffa7fb6a2f462461f5cc98ab26ec5525d6d6ee805a9d5bbd0954ceea7e"
	case "amd64":
		sha = "3dca22e4c2658bec40f38b9c0944342cc42f3980fcb203aac94b96fefc37cb59"
	}

	// Test with command from https://docs.zarf.dev/getting-started/install/
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", fmt.Sprintf("oci://🦄/dos-games:1.0.0-%s@sha256:%s", e2e.Arch, sha), "--key=https://zarf.dev/cosign.pub", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: Cosign deploy")

	// Test with command from https://docs.zarf.dev/getting-started/install/
	command := fmt.Sprintf("%s package deploy sget://defenseunicorns/zarf-hello-world:$(uname -m) --confirm", e2e.ZarfBinPath)

	stdOut, stdErr, err := exec.CmdWithTesting(t, exec.PrintCfg(), "sh", "-c", command)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
