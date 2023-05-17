// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external_test provides a test for the external init flow.
package external_test

import (
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

func TestExtOutClusterDeploy(t *testing.T) {
	// Docker/k3d networking constants
	network := "k3d-k3s-external-test"
	subnet := "172.31.0.0/16"
	gateway := "172.31.0.1"
	giteaIP := "172.31.0.99"
	giteaHost := "gitea.localhost"
	registryHost := "registry.localhost"

	zarfBinPath := path.Join("../../../build", test.GetCLIName())

	// Teardown any leftovers from previous tests
	_ = exec.CmdWithPrint("k3d", "cluster", "delete")
	_ = exec.CmdWithPrint("k3d", "registry", "delete", registryHost)
	_ = exec.CmdWithPrint("docker", "network", "remove", network)

	// Setup a network for everything to live inside
	err := exec.CmdWithPrint("docker", "network", "create", "--driver=bridge", "--subnet="+subnet, "--gateway="+gateway, network)
	require.NoError(t, err, "unable to create the k3d registry")

	// Install a k3d-managed registry server to act as the 'remote' container registry
	err = exec.CmdWithPrint("k3d", "registry", "create", registryHost, "--port", "5000")
	require.NoError(t, err, "unable to create the k3d registry")

	// Create a k3d cluster with the proper networking and aliases
	err = exec.CmdWithPrint("k3d", "cluster", "create", "--registry-use", "k3d-"+registryHost+":5000", "--host-alias", giteaIP+":"+giteaHost, "--network", network)
	require.NoError(t, err, "unable to create the k3d cluster")

	// Install a gitea server via docker compose to act as the 'remote' git server
	err = exec.CmdWithPrint("docker", "compose", "up", "-d")
	require.NoError(t, err, "unable to install the gitea-server")

	// Wait for gitea to deploy properly
	giteaArgs := []string{"inspect", "-f", "{{.State.Status}}", "gitea.init"}
	giteaErrStr := "unable to verify the gitea container installed successfully"
	success := verifyWaitSuccess(t, 2, "docker", giteaArgs, "exited", giteaErrStr)
	require.True(t, success, giteaErrStr)

	// Connect gitea to the k3d network
	err = exec.CmdWithPrint("docker", "network", "connect", "--ip", giteaIP, network, giteaHost)
	require.NoError(t, err, "unable to connect the gitea-server top k3d")

	// Use Zarf to initialize the cluster
	initArgs := []string{"init",
		"--git-push-username=git-user",
		"--git-push-password=superSecurePassword",
		"--git-url=http://" + giteaHost + ":3000",
		"--registry-push-username=git-user",
		"--registry-push-password=superSecurePassword",
		"--registry-url=k3d-" + registryHost + ":5000",
		"--confirm"}
	err = exec.CmdWithPrint(zarfBinPath, initArgs...)

	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-podinfo-flux-amd64.tar.zst", "--confirm"}
	err = exec.CmdWithPrint(zarfBinPath, deployArgs...)

	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pull from the 'external' repository
	podinfoArgs := []string{"wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s"}
	errorStr := "unable to verify flux deployed the podinfo example"
	success = verifyKubectlWaitSuccess(t, 2, podinfoArgs, errorStr)
	require.True(t, success, errorStr)

	// Tear down all of that stuff we made for local runs
	err = exec.CmdWithPrint("k3d", "cluster", "delete")
	require.NoError(t, err, "unable to teardown zarf")

	err = exec.CmdWithPrint("docker", "compose", "down")
	require.NoError(t, err, "unable to teardown the gitea-server")

	err = exec.CmdWithPrint("k3d", "registry", "delete", registryHost)
	require.NoError(t, err, "unable to teardown the k3d registry")

	err = exec.CmdWithPrint("docker", "network", "remove", network)
	require.NoError(t, err, "unable to teardown the docker test network")
}
