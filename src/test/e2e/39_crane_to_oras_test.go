// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

func TestCraneToORAS(t *testing.T) {
	t.Log("E2E: Component Status")

	registryPassword, _, err := e2e.Zarf(t, "tools", "get-creds", "registry")
	require.NoError(t, err)

	var mirrorResourcesCreds = []string{
		"--registry-push-username=zarf-push",
		fmt.Sprintf("--registry-push-password=%s", strings.TrimSpace(registryPassword)),
		"--registry-url=http://zarf-docker-registry.zarf.svc.cluster.local:5000",
	}

	// A package is created with Crane
	cranePkgDir := t.TempDir()
	craneZarfPath := e2e.GetZarfAtVersion(t, "v0.49.1")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(craneZarfPath))
	}()
	cfg := exec.PrintCfg()
	pkgDefinitionPath := filepath.Join("src", "test", "packages", "39-crane-to-oras")
	_, _, err = exec.CmdWithTesting(t, cfg, craneZarfPath, "package", "create", pkgDefinitionPath, "-o", cranePkgDir, "--skip-sbom", "--no-color")
	require.NoError(t, err)

	// Then pushed to the registry using ORAS
	packageName := fmt.Sprintf("zarf-package-images-%s.tar.zst", e2e.Arch)
	cranePkgPath := filepath.Join(cranePkgDir, packageName)
	_, _, err = e2e.Zarf(t, "package", "deploy", cranePkgPath, "--confirm")
	require.NoError(t, err)
	cranePkgMirrorResourcesArgs := []string{"package", "mirror-resources", cranePkgPath, "--confirm"}
	cranePkgMirrorResourcesArgs = append(cranePkgMirrorResourcesArgs, mirrorResourcesCreds...)
	_, _, err = e2e.Zarf(t, cranePkgMirrorResourcesArgs...)
	require.NoError(t, err)

	// A package is created with ORAS
	ORASPkgDir := t.TempDir()
	_, _, err = e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", ORASPkgDir, "--skip-sbom")
	require.NoError(t, err)
	orasPkgPath := filepath.Join(ORASPkgDir, packageName)

	// Then pushed to the registry with Crane
	_, _, err = exec.CmdWithTesting(t, cfg, craneZarfPath, "package", "deploy", orasPkgPath, "--confirm", "--no-color")
	require.NoError(t, err)
	ORASPkgMirrorResourcesArgs := []string{"package", "mirror-resources", orasPkgPath, "--confirm", "--no-color"}
	ORASPkgMirrorResourcesArgs = append(ORASPkgMirrorResourcesArgs, mirrorResourcesCreds...)
	_, _, err = exec.CmdWithTesting(t, cfg, craneZarfPath, ORASPkgMirrorResourcesArgs...)
	require.NoError(t, err)
}
