// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var helmChartsPkg = filepath.Join("build", fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))

func TestHelm(t *testing.T) {
	t.Log("E2E: Helm chart")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	testHelmReleaseName(t)

	testHelmGitChartWithRegistryOverride(t)

	testHelmLocalChart(t)

	testHelmEscaping(t)

	testHelmOCIChart(t)

	testHelmUninstallRollback(t)

	testHelmAdoption(t)
}

func cleanupHelm(t *testing.T) {
	// Remove the package.
	stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "remove", "helm-charts")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmReleaseName(t *testing.T) {
	t.Log("E2E: Helm chart releasename")

	// Deploy the package.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", helmChartsPkg, "--components=demo-helm-alt-release-name")
	require.NoError(t, err, stdOut, stdErr)

	// Verify multiple helm installs of different release names were deployed.
	kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-n=helm-alt-release-name", "--no-headers").Output()
	require.Contains(t, string(kubectlOut), "cool-name-podinfo")

	// Remove the package.
	cleanupHelm(t)
}

func testHelmGitChartWithRegistryOverride(t *testing.T) {
	t.Log("E2E: Git Helm chart w/Registry Override")

	// Create the package.
	stdOut, stdErr, err := e2e.Zarf("package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io=docker.io", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", helmChartsPkg, "--components=demo-helm-git-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, string(stdErr), "registryOverrides", "registry overrides was not saved to build data")
	require.Contains(t, string(stdErr), "docker.io", "docker.io not found in registry overrides")

	// Remove the package.
	cleanupHelm(t)
}

func testHelmLocalChart(t *testing.T) {
	t.Log("E2E: Local Helm chart")

	// Deploy the package.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", helmChartsPkg, "--components=demo-helm-local-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the package.
	cleanupHelm(t)
}

func testHelmEscaping(t *testing.T) {
	t.Log("E2E: Helm chart escaping")

	// Create the package.
	stdOut, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/25-evil-templates/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("zarf-package-evil-templates-%s.tar.zst", e2e.Arch)

	// Deploy the package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was deployed and escaped.
	kubectlOut, _ := exec.Command("kubectl", "describe", "cm", "dont-template-me").Output()
	require.Contains(t, string(kubectlOut), `alert: OOMKilled {{ "{{ \"random.Values\" }}" }}`)
	require.Contains(t, string(kubectlOut), "backtick1: \"content with backticks `some random things`\"")
	require.Contains(t, string(kubectlOut), "backtick2: \"nested templating with backticks {{` random.Values `}}\"")
	require.Contains(t, string(kubectlOut), `description: Pod {{$labels.pod}} in {{$labels.namespace}} got OOMKilled`)

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "evil-templates", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmOCIChart(t *testing.T) {
	t.Log("E2E: Helm OCI chart")

	// Deploy the package.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", helmChartsPkg, "--components=demo-helm-oci-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify that podinfo successfully deploys in the cluster
	kubectlOut, _, _ := e2e.Zarf("tools", "kubectl", "-n=helm-oci-demo", "get", "deployment", "podinfo", "-o=jsonpath={.metadata.labels}")
	require.Contains(t, string(kubectlOut), "6.3.5")

	// Remove the package.
	cleanupHelm(t)
}

func testHelmUninstallRollback(t *testing.T) {
	t.Log("E2E: Helm Uninstall and Rollback")

	goodPath := fmt.Sprintf("build/zarf-package-dos-games-%s.tar.zst", e2e.Arch)
	evilPath := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	// Create the evil package (with the bad configmap).
	stdOut, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/25-evil-dos-games/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that this does not leave behind a dos-games chart
	helmOut, err := exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.NotContains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the good package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", goodPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that this does create a dos-games chart
	helmOut, err = exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that the dos-games chart was not uninstalled
	helmOut, err = exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmAdoption(t *testing.T) {
	t.Log("E2E: Helm Adopt a Deployment")

	packagePath := fmt.Sprintf("build/zarf-package-dos-games-%s.tar.zst", e2e.Arch)
	deploymentManifest := "src/test/packages/25-manifest-adoption/deployment.yaml"

	// Deploy dos-games manually into the cluster without Zarf
	kubectlOut, _, _ := e2e.Zarf("tools", "kubectl", "apply", "-f", deploymentManifest)
	require.Contains(t, string(kubectlOut), "deployment.apps/game created")

	// Deploy dos-games into the cluster with Zarf
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", packagePath, "--confirm", "--adopt-existing-resources")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that this does create a dos-games chart
	helmOut, err := exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
