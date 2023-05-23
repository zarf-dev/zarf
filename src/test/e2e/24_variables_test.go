// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVariables(t *testing.T) {
	t.Log("E2E: Package variables")
	e2e.SetupWithCluster(t)

	path := fmt.Sprintf("build/zarf-package-variables-%s.tar.zst", e2e.Arch)
	tfPath := "modified-terraform.tf"

	e2e.CleanFiles(tfPath)

	// Test that not specifying a prompted variable results in an error
	_, stdErr, _ := e2e.Zarf("package", "deploy", path, "--confirm")
	expectedOutString := "variable 'SITE_NAME' must be '--set' when using the '--confirm' flag"
	require.Contains(t, stdErr, "", expectedOutString)

	// Deploy nginx
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm", "--set", "SITE_NAME=Lula Web", "--set", "AWS_REGION=unicorn-land", "-l", "trace")
	require.NoError(t, err, stdOut, stdErr)
	// Verify that the sensitive variable 'unicorn-land' was not printed to the screen
	require.NotContains(t, stdErr, "unicorn-land")

	logText := e2e.GetLogFileContents(t, stdErr)
	// Verify that the sensitive variable 'unicorn-land' was not included in the log
	require.NotContains(t, logText, "unicorn-land")

	// Verify the terraform file was templated correctly
	outputTF, err := os.ReadFile(tfPath)
	require.NoError(t, err)
	require.Contains(t, string(outputTF), "unicorn-land")

	// Verify the configmap was properly templated
	kubectlOut, _, _ := e2e.Zarf("tools", "kubectl", "-n", "nginx", "get", "configmap", "nginx-configmap", "-o", "jsonpath='{.data.index\\.html}' ")
	// OPTIONAL_FOOTER should remain unset because it was not set during deploy
	require.Contains(t, string(kubectlOut), "</pre>\n    \n  </body>")
	// STYLE should take the default value
	require.Contains(t, string(kubectlOut), "body { font-family: sans-serif;")
	// SITE_NAME should take the set value
	require.Contains(t, string(kubectlOut), "Lula Web")
	// ORGANIZATION should take the created value
	require.Contains(t, string(kubectlOut), "Defense Unicorns")
	// AWS_REGION should have been templated and also templated into this config map
	require.Contains(t, string(kubectlOut), "unicorn-land")

	stdOut, stdErr, err = e2e.Zarf("package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(tfPath)
}
