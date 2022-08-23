package external_test

import (
	"context"
	"fmt"
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

	// Add private-git-server secret to git-server namespace
	secretFilePath := "secret.yaml"
	applyArgs := []string{"apply", fmt.Sprintf("-f=%s", secretFilePath)}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "kubectl", applyArgs...)
	require.NoError(t, err, "unable to apply private-git-server secret ")

	helmAddArgs := []string{"repo", "add", "twuni", "https://helm.twun.io"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "helm", helmAddArgs...)
	require.NoError(t, err, "unable to add the docker-registry chart repo")

	// Install docker-registry chart to the k8s cluster to act as the 'remote' container registry
	helmInstallArgs = []string{"install", "external-registry", "twuni/docker-registry", "-f=docker-registry-values.yaml", "-n=external-registry", "--create-namespace"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "helm", helmInstallArgs...)
	require.NoError(t, err, "unable to install the docker-registry chart")

	// Use Zarf to initialize the cluster
	initArgs := []string{"init",
		"--git-push-username=git-user",
		"--git-push-password=superSecurePassword",
		"--git-url=http://gitea-http.git-server.svc.cluster.local",
		"--git-port=3000",
		"--registry-push-username=push-user",
		"--registry-push-password=superSecurePassword",
		"--registry-url=http://external-registry-docker-registry.external-registry.svc.cluster.local:5000",
		"--nodeport=31999",
		"--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, initArgs...)
	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-flux-test-amd64.tar.zst", "--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, zarfBinPath, deployArgs...)
	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pull from the 'external' repository
	kubectlOut := verifyPodinfoDeployment(t)
	assert.Contains(t, string(kubectlOut), "condition met")
}

func verifyPodinfoDeployment(t *testing.T) string {
	timeout := time.After(1 * time.Minute)
	for {
		// delay check 3 seconds
		time.Sleep(2 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			t.Error("Timeout waiting for flux podinfo deployment")

			// after delay, try running
		default:
			// Check that flux deployed the podinfo example
			kubectlOut, err := exec.Command("kubectl", "wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s").Output()
			// Log error
			if err != nil {
				t.Log(string(kubectlOut), err)
			} else if strings.Contains(string(kubectlOut), "condition met") {
				return string(kubectlOut)
			}
		}
	}
}
