// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package external_test provides a test for the external init flow.
package external_test

import (
	"context"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	test "github.com/defenseunicorns/zarf/src/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtOutClusterDeploy(t *testing.T) {
	zarfBinPath := path.Join("../../../build", test.GetCLIName())

	// Install a gitea server via docker compose to act as the 'remote' git server
	_, _, err := utils.ExecCommandWithContext(context.TODO(), true, "docker", "compose", "up", "-d")
	require.NoError(t, err, "unable to install the gitea-server")

	giteaArgs := []string{"inspect", "-f", "{{.State.Status}}", "gitea-init"}
	giteaErrStr := "unable to verify the gitea container installed successfully"
	success := verifyWaitSuccess(t, 2, "docker", giteaArgs, "exited", giteaErrStr)
	require.True(t, success, giteaErrStr)

	// TODO: (@WSTARR) Make this networking actually work
	// Use Zarf to initialize the cluster
	initArgs := []string{"init",
		"--git-push-username=git-user",
		"--git-push-password=superSecurePassword",
		"--git-url=http://host.k3d.internal:3000",
		"--registry-push-username=git-user",
		"--registry-push-password=superSecurePassword",
		"--registry-url=kubernetes.docker.internal:3000/git-user",
		"--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, initArgs...)

	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-flux-test-amd64.tar.zst", "--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, deployArgs...)

	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pull from the 'external' repository
	podinfoArgs := []string{"wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s"}
	errorStr := "unable to verify flux deployed the podinfo example"
	success = verifyKubectlWaitSuccess(t, 2, podinfoArgs, errorStr)
	assert.True(t, success, errorStr)

	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "docker", "compose", "down")
	require.NoError(t, err, "unable to teardown the gitea-server")

	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, "destroy", "--confirm")
	require.NoError(t, err, "unable to teardown zarf")
}
