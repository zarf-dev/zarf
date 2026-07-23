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

// TestKindListAnchor is a regression test for #4977: a chart rendering a
// `kind: List` whose items share a YAML anchor used to fail post-render with
// "unknown anchor 'shared' referenced" under Helm v4's annotateAndMerge.
func TestKindListAnchor(t *testing.T) {
	t.Log("E2E: kind: List with cross-item YAML anchor")
	tmpdir := t.TempDir()

	pkgPath := filepath.Join("src", "test", "packages", "27-kind-list-anchor")
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", pkgPath, "-o", tmpdir, "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	packagePath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-kind-list-anchor-%s-0.0.1.tar.zst", e2e.Arch))
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Both ConfigMaps should resolve the anchor to data.key = value.
	kubectlOut, _, err := e2e.Kubectl(t, "get", "configmap", "cm-a", "cm-b", "-n", "kind-list-anchor",
		"-o", "jsonpath={range .items[*]}{.metadata.name}={.data.key} {end}")
	require.NoError(t, err, kubectlOut)
	require.Contains(t, kubectlOut, "cm-a=value")
	require.Contains(t, kubectlOut, "cm-b=value")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "kind-list-anchor", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
