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

func TestTarballImageReferences(t *testing.T) {
	t.Log("E2E: Create and deploy a package created with an image referencing a tarball")

	var (
		createPath = filepath.Join("src", "test", "packages", "36-tarball-image-references")
		tmpdir     = t.TempDir()
		tb         = filepath.Join(tmpdir, fmt.Sprintf("zarf-package-tarball-image-reference-%s.tar.zst", e2e.Arch))
	)

	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--confirm", "--output", tmpdir, "--log-level", "debug")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", tb, "--confirm", "--log-level", "debug")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(tb)
}
