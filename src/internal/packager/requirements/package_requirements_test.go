// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestValidatePackageRequirements_NoRequirementsFile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pkgLayout := newTempPackageLayout(ctx, t)

	// No REQUIREMENTS file present => no-op / no error
	err := ValidatePackageRequirements(ctx, pkgLayout)
	require.NoError(t, err)
}

func TestValidatePackageRequirements_InvalidYAML(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pkgLayout := newTempPackageLayout(ctx, t)

	writeFile(t, filepath.Join(pkgLayout.DirPath(), layout.Requirements), []byte("::: this is not yaml :::"))

	err := ValidatePackageRequirements(ctx, pkgLayout)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse requirements.yaml")
}

func TestValidatePackageRequirements_AgentToolMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pkgLayout := newTempPackageLayout(ctx, t)

	writeFile(t, filepath.Join(pkgLayout.DirPath(), layout.Requirements), []byte(`
agent:
  tools:
    - name: definitely-not-a-real-binary-12345
      version: ">= 1.0.0"
`))

	err := ValidatePackageRequirements(ctx, pkgLayout)
	require.Error(t, err)

	var reqErr *requirementsValidationError
	require.ErrorAs(t, err, &reqErr)
	require.Contains(t, err.Error(), "agent tool")
	require.Contains(t, err.Error(), "missing")
}

func TestValidatePackageRequirements_AgentToolVersionMet(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("this test uses a *nix shell script; provide a .bat equivalent if needed")
	}

	ctx := context.Background()
	pkgLayout := newTempPackageLayout(ctx, t)

	toolDir := t.TempDir()
	fakeYQ := filepath.Join(toolDir, "yq")

	// Fake yq that prints a parseable version.
	writeExecutable(t, fakeYQ, []byte(`#!/bin/sh
echo "yq version v4.40.6"
`))

	// Prepend toolDir to PATH so exec.LookPath("yq") finds our fake binary.
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeFile(t, filepath.Join(pkgLayout.DirPath(), layout.Requirements), []byte(`
agent:
  tools:
    - name: yq
      version: ">= 4.40.5"
`))

	err := ValidatePackageRequirements(ctx, pkgLayout)
	require.NoError(t, err)
}

func TestValidatePackageRequirements_AgentToolVersionNotMet(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("this test uses a *nix shell script; provide a .bat equivalent if needed")
	}

	ctx := context.Background()
	pkgLayout := newTempPackageLayout(ctx, t)

	toolDir := t.TempDir()
	fakeYQ := filepath.Join(toolDir, "yq")

	writeExecutable(t, fakeYQ, []byte(`#!/bin/sh
echo "yq version v4.40.4"
`))

	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeFile(t, filepath.Join(pkgLayout.DirPath(), layout.Requirements), []byte(`
agent:
  tools:
    - name: yq
      version: ">= 4.40.5"
`))

	err := ValidatePackageRequirements(ctx, pkgLayout)

	var reqErr *requirementsValidationError
	require.ErrorAs(t, err, &reqErr)
	require.Contains(t, err.Error(), "does not satisfy constraint")
}

func newTempPackageLayout(ctx context.Context, t *testing.T) *layout.PackageLayout {
	t.Helper()

	dir := t.TempDir()

	// Minimal package definition that pkgcfg.Parse accepts when LoadFromDir runs.
	// (Keep it intentionally tiny to ensure unit tests are fast.)
	writeFile(t, filepath.Join(dir, layout.ZarfYAML), []byte(`
kind: ZarfPackageConfig
metadata:
  name: test
  version: 0.0.0
components: []
`))

	pkgLayout, err := layout.LoadFromDir(ctx, dir, layout.PackageLayoutOptions{
		VerificationStrategy: layout.VerifyNever,
	})
	require.NoError(t, err)

	return pkgLayout
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, b, 0o600))
}

func writeExecutable(t *testing.T, path string, b []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, b, 0o700))
}
