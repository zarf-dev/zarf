// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package upgrade provides a test for the upgrade flow.
package upgrade

import (
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

func kubectl(args ...string) (string, string, error) {
	tk := []string{"tools", "kubectl"}
	args = append(tk, args...)
	return zarf(args...)
}

func zarf(args ...string) (string, string, error) {
	zarfBinPath := path.Join("../../../build", test.GetCLIName())
	return exec.Cmd(zarfBinPath, args...)
}

func TestPreviouslyBuiltZarfPackage(t *testing.T) {
	// This test tests that a package built with the previous version of zarf will still deploy with the newer version
	t.Log("Upgrade: Previously Built Zarf Package")

	// For the upgrade test, podinfo-upgrade should already be in the cluster (version 6.3.3) (see .github/workflows/test-upgrade.yml)
	kubeCtlRolloutArgs := []string{"-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade"}
	kubectlOut, _, _ := kubectl(kubeCtlRolloutArgs...)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubeCtlGetArgs := []string{"-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}"}
	kubectlOut, _, _ = kubectl(kubeCtlGetArgs...)
	require.Contains(t, kubectlOut, "6.3.3")

	// We also expect a 6.3.4 package to have been previously built
	previouslyBuiltPackage := "../../../zarf-package-test-upgrade-package-amd64-6.3.4.tar.zst"

	// Deploy the package.
	zarfDeployArgs := []string{"package", "deploy", previouslyBuiltPackage, "--confirm"}
	stdOut, stdErr, err := zarf(zarfDeployArgs...)
	require.NoError(t, err, stdOut, stdErr)

	// [DEPRECATIONS] We expect any deprecated things to work from the old package
	require.Contains(t, stdErr, "Successfully deployed podinfo 6.3.4")
	require.Contains(t, stdErr, "-----BEGIN PUBLIC KEY-----")

	// Verify that podinfo-upgrade successfully deploys in the cluster (version 6.3.4)
	kubeCtlRolloutArgs = []string{"-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade"}
	kubectlOut, _, _ = kubectl(kubeCtlRolloutArgs...)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubeCtlGetArgs = []string{"-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}"}
	kubectlOut, _, _ = kubectl(kubeCtlGetArgs...)
	require.Contains(t, kubectlOut, "6.3.4")

	// We also want to build a new package.
	zarfCreateArgs := []string{"package", "create", "../../../src/test/upgrade", "--set", "PODINFO_VERSION=6.3.5", "--confirm"}
	stdOut, stdErr, err = zarf(zarfCreateArgs...)
	require.NoError(t, err, stdOut, stdErr)
	newlyBuiltPackage := "zarf-package-test-upgrade-package-amd64-6.3.5.tar.zst"

	// Deploy the package.
	zarfDeployArgs = []string{"package", "deploy", newlyBuiltPackage, "--confirm"}
	stdOut, stdErr, err = zarf(zarfDeployArgs...)
	require.NoError(t, err, stdOut, stdErr)

	// [DEPRECATIONS] We expect any deprecated things to work from the new package
	require.Contains(t, stdErr, "Successfully deployed podinfo 6.3.5")
	require.Contains(t, stdErr, "-----BEGIN PUBLIC KEY-----")

	// Verify that podinfo-upgrade successfully deploys in the cluster (version 6.3.5)
	kubeCtlRolloutArgs = []string{"-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade"}
	kubectlOut, _, _ = kubectl(kubeCtlRolloutArgs...)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubeCtlGetArgs = []string{"-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}"}
	kubectlOut, _, _ = kubectl(kubeCtlGetArgs...)
	require.Contains(t, kubectlOut, "6.3.5")

	// Remove the package.
	zarfRemoveArgs := []string{"package", "remove", "test-upgrade-package", "--confirm"}
	stdOut, stdErr, err = zarf(zarfRemoveArgs...)
	require.NoError(t, err, stdOut, stdErr)
}
