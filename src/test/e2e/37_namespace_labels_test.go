// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNamespaceLabels(t *testing.T) {
	chartsNamespace := "namespace-labels-charts"
	manifestsNamespace := "namespace-labels-manifests"

	testNamespaceLabelsForNamespace(t, chartsNamespace, "0.0.1")
	testNamespaceLabelsForNamespace(t, manifestsNamespace, "")
}

func testNamespaceLabelsForNamespace(t *testing.T, namespace, packageSuffix string) {
	defer e2e.Kubectl("delete", "namespace", namespace)
	testLabelKey := "testlabelkey"
	testLabelValue := "testlabelvalue"
	packageName := ""

	t.Log("E2E: Namespace Labels")
	e2e.SetupWithCluster(t)
	buildPath := filepath.Join("src", "test", "packages", fmt.Sprintf("37-%s", namespace))

	if packageSuffix != "" {
		packageName = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", namespace, e2e.Arch, packageSuffix)
	} else {
		packageName = fmt.Sprintf("zarf-package-%s-%s.tar.zst", namespace, e2e.Arch)
	}

	stdOut, stdErr, err := e2e.Zarf("package", "create", buildPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(packageName)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", packageName, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl("get", "namespace", namespace, "--show-labels")
	t.Log(stdOut)
	require.Contains(t, stdOut, fmt.Sprintf("%s=%s", testLabelKey, testLabelValue))
}
