package external_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalDeploy(t *testing.T) {

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

	// Use Zarf to initialize the cluster
	initArgs := []string{"init", "--git-user", "git-user", "--git-password", "superSecurePassword", "--git-url", "http://gitea-http.git-server.svc.cluster.local:3000", "--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "zarf", initArgs...)
	require.NoError(t, err, "unable to initialize the k8s server with zarf")

	// Deploy the flux example package
	deployArgs := []string{"package", "deploy", "../../../build/zarf-package-flux-test-amd64.tar.zst", "--confirm"}
	_, _, err = utils.ExecCommandWithContext(context.TODO(), true, "zarf", deployArgs...)
	require.NoError(t, err, "unable to deploy flux example package")

	// Verify flux was able to pulll from the 'external' registry
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
