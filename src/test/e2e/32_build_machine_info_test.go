// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test"
)

func TestIncludedBuildMachineInfo(t *testing.T) {
	t.Log("E2E: Included Build Machine Info")
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))

	tmpdir := t.TempDir()

	packagePath := "examples/dos-games"
	packageName := fmt.Sprintf("zarf-package-dos-games-%s-1.2.0.tar.zst", e2e.Arch)

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packagePath, "-o", tmpdir, "--with-build-machine-info", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	pkgWithInfo := filepath.Join(tmpdir, packageName)
	pkgLayout, err := layout.LoadFromTar(ctx, pkgWithInfo, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, pkgLayout.Pkg.Build.Terminal)
	require.NotEmpty(t, pkgLayout.Pkg.Build.User)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", pkgWithInfo)
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "terminal:")
	require.Contains(t, stdOut, "user:")
}

func TestOmitteddBuildMachineInfo(t *testing.T) {
	t.Log("E2E: Omitted Build Machine Info")
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))

	packagePath := "examples/dos-games"
	packageName := fmt.Sprintf("zarf-package-dos-games-%s-1.2.0.tar.zst", e2e.Arch)

	tmpdir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packagePath, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	pkgWithoutInfo := filepath.Join(tmpdir, packageName)
	pkgLayout, err := layout.LoadFromTar(ctx, pkgWithoutInfo, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	require.Empty(t, pkgLayout.Pkg.Build.Terminal)
	require.Empty(t, pkgLayout.Pkg.Build.User)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", pkgWithoutInfo)
	require.NoError(t, err, stdOut, stdErr)
	require.NotContains(t, stdOut, "terminal:")
	require.NotContains(t, stdOut, "user:")
}
