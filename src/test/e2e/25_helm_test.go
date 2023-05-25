// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var helmChartsPkg string

func TestHelm(t *testing.T) {
	t.Log("E2E: Helm chart")
	e2e.SetupWithCluster(t)

	helmChartsPkg = filepath.Join("build", fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))

	testHelmChartsExample(t)

	testHelmEscaping(t)

	testHelmUninstallRollback(t)

	testHelmAdoption(t)
}

func testHelmChartsExample(t *testing.T) {

	// Create the package with a registry override
	stdOut, stdErr, err := e2e.Zarf("package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io=docker.io", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the package.
	allComponents := []string{
		"demo-helm-local-chart",
		"demo-helm-git-chart",
		"demo-helm-oci-chart",
		"demo-helm-alt-release-name",
	}
	componentsFlag := fmt.Sprintf("--components=%s", strings.Join(allComponents, ","))
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", helmChartsPkg, componentsFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, string(stdErr), "registryOverrides", "registry overrides was not saved to build data")
	require.Contains(t, string(stdErr), "docker.io", "docker.io not found in registry overrides")

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "helm-charts", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
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
	kubectlOut, _, _ := e2e.Kubectl("apply", "-f", deploymentManifest)
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
