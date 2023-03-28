// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelm(t *testing.T) {
	t.Log("E2E: Helm chart")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	testHelmReleaseName(t)

	testHelmLocalChart(t)

	testHelmEscaping(t)

	testHelmOCIChart(t)

	testHelmUninstallRollback(t)
}

func testHelmReleaseName(t *testing.T) {
	t.Log("E2E: Helm chart releasename")

	path := fmt.Sprintf("build/zarf-package-test-helm-releasename-%s.tar.zst", e2e.arch)

	// Deploy the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify multiple helm installs of different release names were deployed.
	kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-n=helm-releasename", "--no-headers").Output()
	assert.Contains(t, string(kubectlOut), "cool-name-podinfo")

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "test-helm-releasename", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmLocalChart(t *testing.T) {
	t.Log("E2E: Local Helm chart")

	path := fmt.Sprintf("build/zarf-package-test-helm-local-chart-%s.tar.zst", e2e.arch)

	// Deploy the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify that nginx successfully deploys in the cluster
	kubectlOut, _, _ := e2e.execZarfCommand("tools", "kubectl", "-n=local-chart", "rollout", "status", "deployment/local-demo")
	assert.Contains(t, string(kubectlOut), "successfully rolled out")

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "test-helm-local-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmEscaping(t *testing.T) {
	t.Log("E2E: Helm chart escaping")

	// Create the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "src/test/test-packages/25-evil-templates/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("zarf-package-evil-templates-%s.tar.zst", e2e.arch)

	// Deploy the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was deployed and escaped.
	kubectlOut, _ := exec.Command("kubectl", "describe", "cm", "dont-template-me").Output()
	assert.Contains(t, string(kubectlOut), `alert: OOMKilled {{ "{{ \"random.Values\" }}" }}`)
	assert.Contains(t, string(kubectlOut), "backtick1: \"content with backticks `some random things`\"")
	assert.Contains(t, string(kubectlOut), "backtick2: \"nested templating with backticks {{` random.Values `}}\"")
	assert.Contains(t, string(kubectlOut), `description: Pod {{$labels.pod}} in {{$labels.namespace}} got OOMKilled`)

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "evil-templates", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmOCIChart(t *testing.T) {
	t.Log("E2E: Helm OCI chart")

	path := fmt.Sprintf("build/zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.arch)

	// Deploy the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify that podinfo successfully deploys in the cluster
	kubectlOut, _, _ := e2e.execZarfCommand("tools", "kubectl", "-n=helm-oci-demo", "rollout", "status", "deployment/podinfo")
	assert.Contains(t, string(kubectlOut), "successfully rolled out")
	kubectlOut, _, _ = e2e.execZarfCommand("tools", "kubectl", "-n=helm-oci-demo", "get", "deployment", "podinfo", "-o=jsonpath={.metadata.labels}")
	assert.Contains(t, string(kubectlOut), "6.3.3")

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "helm-oci-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmUninstallRollback(t *testing.T) {
	t.Log("E2E: Helm Uninstall and Rollback")

	goodPath := fmt.Sprintf("build/zarf-package-dos-games-%s.tar.zst", e2e.arch)
	evilPath := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.arch)

	// Create the evil package (with the bad configmap).
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "src/test/test-packages/25-evil-dos-games/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that this does not leave behind a dos-games chart
	helmOut, err := exec.Command("helm", "list", "-n", "zarf").Output()
	require.NoError(t, err)
	assert.NotContains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the good package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", goodPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that this does create a dos-games chart
	helmOut, err = exec.Command("helm", "list", "-n", "zarf").Output()
	require.NoError(t, err)
	assert.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that the dos-games chart was not uninstalled
	helmOut, err = exec.Command("helm", "list", "-n", "zarf").Output()
	require.NoError(t, err)
	assert.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
