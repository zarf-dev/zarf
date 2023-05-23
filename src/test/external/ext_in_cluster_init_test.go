// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external provides a test for the external init flow.
package external

import (
	"context"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

func TestExtInClusterDeploy(t *testing.T) {
	zarfBinPath := path.Join("../../../build", test.GetCLIName())

	// Install a gitea chart to the k8s cluster to act as the 'remote' git server
	giteaChartURL := "https://dl.gitea.io/charts/gitea-5.0.8.tgz"
	helmInstallArgs := []string{"install", "gitea", giteaChartURL, "-f", "gitea-values.yaml", "-n", "git-server", "--create-namespace"}
	err := exec.CmdWithPrint("helm", helmInstallArgs...)
	require.NoError(t, err, "unable to install gitea chart")

	// Install docker-registry chart to the k8s cluster to act as the 'remote' container registry
	helmAddArgs := []string{"repo", "add", "twuni", "https://helm.twun.io"}
	err = exec.CmdWithPrint("helm", helmAddArgs...)
	require.NoError(t, err, "unable to add the docker-registry chart repo")

	helmInstallArgs = []string{"install", "external-registry", "twuni/docker-registry", "-f=docker-registry-values.yaml", "-n=external-registry", "--create-namespace"}
	err = exec.CmdWithPrint("helm", helmInstallArgs...)
	require.NoError(t, err, "unable to install the docker-registry chart")

	// Verify the registry and gitea helm charts installed successfully
	registryWaitCmd := []string{"wait", "deployment", "-n=external-registry", "external-registry-docker-registry", "--for", "condition=Available=True", "--timeout=5s"}
	registryErrStr := "unable to verify the docker-registry chart installed successfully"
	giteaWaitCmd := []string{"wait", "pod", "-n=git-server", "gitea-0", "--for", "condition=Ready=True", "--timeout=5s"}
	giteaErrStr := "unable to verify the gitea chart installed successfully"
	success := verifyKubectlWaitSuccess(t, 2, registryWaitCmd, registryErrStr)
	require.True(t, success, registryErrStr)
	success = verifyKubectlWaitSuccess(t, 3, giteaWaitCmd, giteaErrStr)
	require.True(t, success, giteaErrStr)

	// Use Zarf to initialize the cluster
	initArgs := []string{"init",
		"--git-push-username=git-user",
		"--git-push-password=superSecurePassword",
		"--git-url=http://gitea-http.git-server.svc.cluster.local:3000",
		"--registry-push-username=push-user",
		"--registry-push-password=superSecurePassword",
		"--registry-url=127.0.0.1:31999",
		"--confirm"}

	err = exec.CmdWithPrint(zarfBinPath, initArgs...)

	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-podinfo-flux-amd64.tar.zst", "--confirm"}
	err = exec.CmdWithPrint(zarfBinPath, deployArgs...)

	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pull from the 'external' repository
	podinfoWaitCmd := []string{"wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s"}
	errorStr := "unable to verify flux deployed the podinfo example"
	success = verifyKubectlWaitSuccess(t, 2, podinfoWaitCmd, errorStr)
	require.True(t, success, errorStr)

	_, _, err = exec.CmdWithContext(context.TODO(), exec.PrintCfg(), zarfBinPath, "destroy", "--confirm")
	require.NoError(t, err, "unable to teardown zarf")
}
