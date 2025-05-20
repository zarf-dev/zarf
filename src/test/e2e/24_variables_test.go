// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVariables(t *testing.T) {
	t.Log("E2E: Package variables")

	evilSrc := filepath.Join("src", "test", "packages", "24-evil-variables")
	evilPath := fmt.Sprintf("zarf-package-evil-variables-%s.tar.zst", e2e.Arch)

	src := filepath.Join("examples", "variables")
	path := filepath.Join("build", fmt.Sprintf("zarf-package-variables-%s.tar.zst", e2e.Arch))

	tfPath := "modified-terraform.tf"

	e2e.CleanFiles(t, tfPath, evilPath)

	// Test that specifying an invalid setVariable value results in an error
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", evilSrc, "--set", "NUMB3R5=K1TT3H", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	expectedOutString := "\"K1TT3H\""
	require.Contains(t, stdErr, "", expectedOutString)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", evilPath, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	expectedOutString = "variable \"HELLO_KITTEH\" does not match pattern "
	require.Contains(t, stdErr, "", expectedOutString)

	// Test that specifying an invalid constant value results in an error
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", src, "--set", "NGINX_VERSION=", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	expectedOutString = "constant \"NGINX_VERSION\" does not match pattern "
	require.Contains(t, stdErr, "", expectedOutString)

	// Test that not specifying a prompted variable results in an error
	_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	expectedOutString = "variable 'SITE_NAME' must be '--set' when using the '--confirm' flag"
	require.Error(t, err)
	require.Contains(t, stdErr, "", expectedOutString)

	// Test that specifying an invalid variable value results in an error
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--set", "SITE_NAME=#INVALID", "--confirm")
	require.Error(t, err, stdOut, stdErr)
	expectedOutString = "variable \"SITE_NAME\" does not match pattern "
	require.Contains(t, stdErr, "", expectedOutString)

	// Deploy nginx
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--set", "SITE_NAME=Lula Web", "--set", "AWS_REGION=unicorn-land", "-l", "trace")
	require.NoError(t, err, stdOut, stdErr)
	// Verify that the variables were shown to the user in the formats we expect
	require.Contains(t, stdOut, "currently set to 'Defense Unicorns' (default)")
	require.Contains(t, stdOut, "currently set to 'Lula Web'")
	require.Contains(t, stdOut, "currently set to '**sanitized**'")
	// Verify that the sensitive variable 'unicorn-land' was not printed to the screen
	require.NotContains(t, stdOut, "unicorn-land")

	// Verify the terraform file was templated correctly
	outputTF, err := os.ReadFile(tfPath)
	require.NoError(t, err)
	require.Contains(t, string(outputTF), "unicorn-land")

	// Verify the configmap was properly templated
	kubectlOut, _, err := e2e.Kubectl(t, "-n", "nginx", "get", "configmap", "nginx-configmap", "-o", "jsonpath='{.data.index\\.html}' ")
	require.NoError(t, err, "unable to get nginx configmap")
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
	// MODIFIED_TERRAFORM_SHASUM should have been templated
	require.Contains(t, string(kubectlOut), "63af41aebec53e3679948b254073c3c0d603d47ab01b03ab14abd7d98234e101")

	// Remove the variables example
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(t, tfPath, evilPath)
}
