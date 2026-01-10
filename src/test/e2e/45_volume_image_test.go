// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

func TestVolumeImageMount(t *testing.T) {
	t.Log("E2E: volume image")

	// Skip test if Kubernetes version is less than 1.35.0
	stdOut, _, err := e2e.Kubectl(t, "version", "--output=json")
	require.NoError(t, err)

	var versionInfo struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	err = json.Unmarshal([]byte(stdOut), &versionInfo)
	require.NoError(t, err)

	k8sVersion, err := semver.NewVersion(versionInfo.ServerVersion.GitVersion)
	require.NoError(t, err)

	if k8sVersion.LessThan(semver.MustParse("v1.35.0")) {
		t.Skipf("Skipping test: Kubernetes version %s is less than 1.35.0", versionInfo.ServerVersion.GitVersion)
	} else {
		pkgDefinitionPath := filepath.Join("src", "pkg", "packager", "testdata", "find-images", "pod-volume-image")
		tmpdir := t.TempDir()
		_, _, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", tmpdir)
		require.NoError(t, err)

		packageName := fmt.Sprintf("zarf-package-pod-volume-image-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)

		_, _, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
		require.NoError(t, err)

		stdOut, _, err := e2e.Kubectl(t, "get", "pod", "-l", "app=agent", "-n", "pod-volume-image", "-o", "jsonpath={.items[].spec.volumes[].image.reference}")
		require.NoError(t, err)
		require.Equal(t, "127.0.0.1:31999/zarf-dev/zarf/agent:v0.68.1-zarf-2203613481", stdOut)

		_, _, err = e2e.Zarf(t, "package", "remove", "pod-volume-image", "--confirm")
		require.NoError(t, err)
	}
}
