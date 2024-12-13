// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package upgrade provides a test for the upgrade flow.
package upgrade

import (
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	test "github.com/zarf-dev/zarf/src/test"
)

var zarfBinPath = path.Join("../../../build", test.GetCLIName())

func kubectl(t *testing.T, args ...string) (string, string, error) {
	tk := []string{"tools", "kubectl"}
	args = append(tk, args...)
	return exec.CmdWithTesting(t, exec.PrintCfg(), zarfBinPath, args...)
}

func zarf(t *testing.T, args ...string) (string, string, error) {
	args = append(args, "--log-format=console", "--no-color")
	return exec.CmdWithTesting(t, exec.PrintCfg(), zarfBinPath, args...)
}

func TestPreviouslyBuiltZarfPackage(t *testing.T) {
	// This test tests that a package built with the previous version of zarf will still deploy with the newer version
	t.Log("Upgrade: Previously Built Zarf Package")

	// For the upgrade test, podinfo-upgrade should already be in the cluster (version 6.3.3) (see .github/workflows/test-upgrade.yml)
	kubectlOut, _, err := kubectl(t, "-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubectlOut, _, err = kubectl(t, "-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "6.3.3")

	// Verify that the private-registry secret and private-git-server secret in the podinfo-upgrade namespace are the same after re-init
	// This tests that `zarf tools update-creds` successfully updated the other namespace
	zarfRegistrySecret, _, err := kubectl(t, "-n=zarf", "get", "secret", "private-registry", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	podinfoRegistrySecret, _, err := kubectl(t, "-n=podinfo-upgrade", "get", "secret", "private-registry", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	require.Equal(t, zarfRegistrySecret, podinfoRegistrySecret, "the zarf registry secret and podinfo-upgrade registry secret did not match")
	zarfGitServerSecret, _, err := kubectl(t, "-n=zarf", "get", "secret", "private-git-server", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	podinfoGitServerSecret, _, err := kubectl(t, "-n=podinfo-upgrade", "get", "secret", "private-git-server", "-o", "jsonpath={.data}")
	require.NoError(t, err)
	require.Equal(t, zarfGitServerSecret, podinfoGitServerSecret, "the zarf git server secret and podinfo-upgrade git server secret did not match")

	// We also expect a 6.3.4 package to have been previously built
	previouslyBuiltPackage := "../../../zarf-package-test-upgrade-package-amd64-6.3.4.tar.zst"

	// Deploy the package.
	zarfDeployArgs := []string{"package", "deploy", previouslyBuiltPackage, "--confirm"}
	stdOut, stdErr, err := zarf(t, zarfDeployArgs...)
	require.NoError(t, err, stdOut, stdErr)

	// [DEPRECATIONS] We expect any deprecated things to work from the old package
	require.Contains(t, stdErr, "Successfully deployed podinfo 6.3.4")
	require.Contains(t, stdErr, "-----BEGIN PUBLIC KEY-----")

	// Verify that podinfo-upgrade successfully deploys in the cluster (version 6.3.4)
	kubectlOut, _, err = kubectl(t, "-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubectlOut, _, err = kubectl(t, "-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "6.3.4")

	// We also want to build a new package.
	stdOut, stdErr, err = zarf(t, "package", "create", "../../../src/test/upgrade", "--set", "PODINFO_VERSION=6.3.5", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	newlyBuiltPackage := "zarf-package-test-upgrade-package-amd64-6.3.5.tar.zst"

	// Deploy the package.
	stdOut, stdErr, err = zarf(t, "package", "deploy", newlyBuiltPackage, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// [DEPRECATIONS] We expect any deprecated things to work from the new package
	require.Contains(t, stdErr, "Successfully deployed podinfo 6.3.5")
	require.Contains(t, stdErr, "-----BEGIN PUBLIC KEY-----")

	// Verify that podinfo-upgrade successfully deploys in the cluster (version 6.3.5)
	kubectlOut, _, err = kubectl(t, "-n=podinfo-upgrade", "rollout", "status", "deployment/podinfo-upgrade")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "successfully rolled out")
	kubectlOut, _, err = kubectl(t, "-n=podinfo-upgrade", "get", "deployment", "podinfo-upgrade", "-o=jsonpath={.metadata.labels}}")
	require.NoError(t, err)
	require.Contains(t, kubectlOut, "6.3.5")

	// Remove the package.
	stdOut, stdErr, err = zarf(t, "package", "remove", "test-upgrade-package", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
