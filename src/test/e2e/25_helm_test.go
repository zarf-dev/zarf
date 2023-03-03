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

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "test-helm-local-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func testHelmEscaping(t *testing.T) {
	t.Log("E2E: Helm chart escaping")

	// Create the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "src/test/test-packages/evil-templates/", "--confirm")
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

	// Remove the package.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "helm-oci-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
