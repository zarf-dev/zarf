package external_test

import (
	"context"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	test "github.com/defenseunicorns/zarf/src/test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalDeploy(t *testing.T) {
	zarfBinPath := path.Join("../../../build", test.GetCLIName())

	// Install a gitea chart to the k8s cluster to act as the 'remote' git server
	giteaChartURL := "https://dl.gitea.io/charts/gitea-5.0.8.tgz"
	helmInstallArgs := []string{"install", "gitea", giteaChartURL, "-f", "gitea-values.yaml", "-n", "git-server", "--create-namespace"}
	_, _, err := utils.ExecCommandWithContext(context.TODO(), true, "helm", helmInstallArgs...)
	require.NoError(t, err, "unable to install gitea chart")

	// Install docker-registry chart to the k8s cluster to act as the 'remote' container registry
	helmAddArgs := []string{"repo", "add", "twuni", "https://helm.twun.io"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "helm", helmAddArgs...)
	require.NoError(t, err, "unable to add the docker-registry chart repo")
	helmInstallArgs = []string{"install", "external-registry", "twuni/docker-registry", "-f=docker-registry-values.yaml", "-n=external-registry", "--create-namespace"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "helm", helmInstallArgs...)
	require.NoError(t, err, "unable to install the docker-registry chart")

	// Verify the registry and gitea helm charts installed successfully
	registryWaitCmd := []string{"wait", "deployment", "-n=external-registry", "external-registry-docker-registry", "--for", "condition=Available=True", "--timeout=5s"}
	registryErrStr := "unable to verify the docker-registry chart installed successfully"
	giteaWaitCmd := []string{"wait", "pod", "-n=git-server", "gitea-0", "--for", "condition=Ready=True", "--timeout=5s"}
	giteaErrStr := "unable to verify the gitea chart installed successfully"
	success := verifyKubectlWaitSuccess(t, 2, registryWaitCmd, registryErrStr)
	require.True(t, success, registryErrStr)
	success = verifyKubectlWaitSuccess(t, 2, giteaWaitCmd, giteaErrStr)
	require.True(t, success, giteaErrStr)

	// Use Zarf to initialize the cluster
	initArgs := []string{"init",
		"--git-push-username=git-user",
		"--git-push-password=superSecurePassword",
		"--git-url=http://gitea-http.git-server.svc.cluster.local:3000",
		"--registry-push-username=push-user",
		"--registry-push-password=superSecurePassword",
		"--registry-url=http://external-registry-docker-registry.external-registry.svc.cluster.local:5000",
		"--nodeport=31999",
		"--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, initArgs...)
	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-flux-test-amd64.tar.zst", "--confirm", "-l=trace"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, deployArgs...)
	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pull from the 'external' repository
	podinfoWaitCmd := []string{"wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s"}
	errorStr := "unable to verify flux deployed the podinfo example"
	success = verifyKubectlWaitSuccess(t, 2, podinfoWaitCmd, errorStr)
	assert.True(t, success, errorStr)
}

func verifyKubectlWaitSuccess(t *testing.T, timeoutMinutes time.Duration, waitCmd []string, errorStr string) bool {
	timeout := time.After(timeoutMinutes * time.Minute)
	for {
		// delay check 3 seconds
		time.Sleep(3 * time.Second)
		select {
		// on timeout abort
		case <-timeout:
			t.Error(errorStr)

			// after delay, try running
		default:
			// Check that flux deployed the podinfo example
			kubectlOut, err := exec.Command("kubectl", waitCmd...).Output()
			// Log error
			if err != nil {
				t.Log(string(kubectlOut), err)
			}
			if strings.Contains(string(kubectlOut), "condition met") {
				return true
			}
		}
	}
}
