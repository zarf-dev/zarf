// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVolumeImageMount(t *testing.T) {
	t.Log("E2E: image tar push")

	pkgDefinitionPath := filepath.Join("src", "pkg", "packager", "testdata", "find-images", "pod-volume-image")
	tmpdir := t.TempDir()
	_, _, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", tmpdir)
	require.NoError(t, err)

	packageName := fmt.Sprintf("zarf-package-pod-volume-image-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	_, _, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--skip-version-check")
	require.NoError(t, err)

	stdOut, _, err := e2e.Kubectl(t, "get", "pod", "-l", "app=agent", "-n", "pod-volume-image", "-o", "jsonpath={.status.phase}")
	require.NoError(t, err)
	require.Equal(t, "Running", stdOut)

	_, _, err = e2e.Zarf(t, "package", "remove", "pod-volume-image", "--confirm", "--skip-version-check")
	require.NoError(t, err)
}
