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

func TestHelmReleaseHistory(t *testing.T) {
	outputPath := t.TempDir()
	localTgzChartPath := filepath.Join("src", "test", "packages", "25-helm-release-history")
	_, _, err := e2e.Zarf(t, "package", "create", localTgzChartPath, "-o", outputPath, "--confirm")
	require.NoError(t, err)

	packagePath := filepath.Join(outputPath, fmt.Sprintf("zarf-package-helm-release-history-%s-0.0.1.tar.zst", e2e.Arch))
	for range 20 {
		_, _, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm")
		require.NoError(t, err)
	}

	stdout, err := exec.Command("helm", "history", "-n", "helm-release-history", "chart").Output()
	require.NoError(t, err)
	out := strings.TrimSpace(string(stdout))
	count := len(strings.Split(string(out), "\n"))
	require.Equal(t, 11, count)

	_, _, err = e2e.Zarf(t, "package", "remove", packagePath, "--confirm")
	require.NoError(t, err)
}

func TestHelm(t *testing.T) {
	t.Log("E2E: Helm chart")

	testHelmUninstallRollback(t)

	testHelmAdoption(t)

	t.Run("helm charts example", testHelmChartsExample)

	t.Run("helm escaping", testHelmEscaping)
}

func testHelmChartsExample(t *testing.T) {
	t.Parallel()
	t.Log("E2E: Helm chart example")
	tmpdir := t.TempDir()

	// Create a package that has a tarball as a local chart
	localTgzChartPath := filepath.Join("src", "test", "packages", "25-local-tgz-chart")
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", localTgzChartPath, "--tmpdir", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(t, fmt.Sprintf("zarf-package-helm-charts-local-tgz-%s-0.0.1.tar.zst", e2e.Arch))

	// Create a package that needs dependencies
	evilChartDepsPath := filepath.Join("src", "test", "packages", "25-evil-chart-deps")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", evilChartDepsPath, "--tmpdir", tmpdir, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, e2e.StripMessageFormatting(stdErr), "could not download https://charts.jetstack.io/charts/cert-manager-v1.11.1.tgz")
	require.FileExists(t, filepath.Join(evilChartDepsPath, "good-chart", "charts", "gitlab-runner-0.55.0.tgz"))

	// Create a package with a chart name that doesn't exist in a repo
	evilChartLookupPath := filepath.Join("src", "test", "packages", "25-evil-chart-lookup")
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", evilChartLookupPath, "--tmpdir", tmpdir, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, e2e.StripMessageFormatting(stdErr), "chart \"asdf\" version \"6.4.0\" not found")

	// Create a test package (with a registry override (host+subpath to host+subpath) to test that as well)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io/stefanprodan=docker.io/stefanprodan", "--tmpdir", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Create a test package (with a registry override (host to host+subpath) to test that as well)
	// expect to fail as ghcr.io is overridden and the expected final image doesn't exist but the override works well based on the error message in the output
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io=localhost:555/noway", "--tmpdir", tmpdir, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, string(stdErr), "localhost:555/noway")

	// Create a test package (with a registry override (host+subpath to host) to test that as well)
	// works same as the above failing test
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io/stefanprodan=localhost:555", "--tmpdir", tmpdir, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Contains(t, string(stdErr), "localhost:555")

	// Create the package (with a registry override (host to host) to test that as well)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "examples/helm-charts", "-o", "build", "--registry-override", "ghcr.io=docker.io", "--tmpdir", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the example package.
	helmChartsPkg := filepath.Join("build", fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", helmChartsPkg, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, string(stdErr), "registryOverrides", "registry overrides was not saved to build data")
	require.Contains(t, string(stdErr), "docker.io", "docker.io not found in registry overrides")

	// Remove the example package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "helm-charts", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmEscaping(t *testing.T) {
	t.Parallel()
	t.Log("E2E: Helm chart escaping")

	// Create the package.
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/25-evil-templates/", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("zarf-package-evil-templates-%s.tar.zst", e2e.Arch)

	// Deploy the package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was deployed, escaped, and contains all of its data
	kubectlOut, err := exec.Command("kubectl", "-n", "default", "describe", "cm", "dont-template-me").Output()
	require.NoError(t, err, "unable to describe configmap")
	require.Contains(t, string(kubectlOut), `alert: OOMKilled {{ "{{ \"random.Values\" }}" }}`)
	require.Contains(t, string(kubectlOut), "backtick1: \"content with backticks `some random things`\"")
	require.Contains(t, string(kubectlOut), "backtick2: \"nested templating with backticks {{` random.Values `}}\"")
	require.Contains(t, string(kubectlOut), `description: Pod {{$labels.pod}} in {{$labels.namespace}} got OOMKilled`)
	require.Contains(t, string(kubectlOut), `TG9yZW0gaXBzdW0gZG9sb3Igc2l0IGFtZXQsIGNvbnNlY3RldHVyIG`)

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "evil-templates", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmUninstallRollback(t *testing.T) {
	t.Log("E2E: Helm Uninstall and Rollback")

	goodPath := fmt.Sprintf("build/zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch)
	evilPath := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	// Create the evil package (with the bad service).
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/25-evil-dos-games/", "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", evilPath, "--timeout", "10s", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// This package contains SBOMable things but was created with --skip-sbom
	require.Contains(t, string(stdErr), "This package does NOT contain an SBOM.")

	// Ensure this leaves behind a dos-games chart.
	// We do not want to uninstall charts that had failed installs/upgrades
	// to prevent unintentional deletion and/or data loss in production environments.
	// https://github.com/zarf-dev/zarf/issues/2455
	helmOut, err := exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the good package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", goodPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure this upgrades/fixes the dos-games chart.
	helmOut, err = exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	// Deploy the evil package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", evilPath, "--timeout", "10s", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that we rollback properly
	helmOut, err = exec.Command("helm", "history", "-n", "dos-games", "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866", "--max", "1").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "Rollback to 4")

	// Deploy the evil package (again to ensure we check full history)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", evilPath, "--timeout", "10s", "--confirm")
	require.Error(t, err, stdOut, stdErr)

	// Ensure that we rollback properly
	helmOut, err = exec.Command("helm", "history", "-n", "dos-games", "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866", "--max", "1").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "Rollback to 8")

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmAdoption(t *testing.T) {
	t.Log("E2E: Helm Adopt a Deployment")

	packagePath := fmt.Sprintf("build/zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch)
	deploymentManifest := "src/test/packages/25-manifest-adoption/deployment.yaml"

	// Deploy dos-games manually into the cluster without Zarf
	kubectlOut, _, err := e2e.Kubectl(t, "apply", "-f", deploymentManifest)
	require.NoError(t, err, "unable to apply", "deploymentManifest", deploymentManifest)
	require.Contains(t, kubectlOut, "deployment.apps/game created")

	// Deploy dos-games into the cluster with Zarf
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", packagePath, "--confirm", "--adopt-existing-resources")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure that this does create a dos-games chart
	helmOut, err := exec.Command("helm", "list", "-n", "dos-games").Output()
	require.NoError(t, err)
	require.Contains(t, string(helmOut), "zarf-f53a99d4a4dd9a3575bedf59cd42d48d751ae866")

	existingLabel, _, err := e2e.Kubectl(t, "get", "ns", "dos-games", "-o=jsonpath={.metadata.labels.keep-this}")
	require.Equal(t, "label", existingLabel)
	require.NoError(t, err)
	existingAnnotation, _, err := e2e.Kubectl(t, "get", "ns", "dos-games", "-o=jsonpath={.metadata.annotations.keep-this}")
	require.Equal(t, "annotation", existingAnnotation)
	require.NoError(t, err)

	// Remove the package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
