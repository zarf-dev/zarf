package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageVariables(t *testing.T) {
	t.Log("E2E: Package variables")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-package-variables-%s.tar.zst", e2e.arch)

	// Test that not specifying a prompted variable results in an error
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	expectedOutString := "variable 'STANDARD' has no 'default'"
	require.Contains(t, stdErr, expectedOutString)
	require.Error(t, err, stdOut, stdErr)

	// Deploy the simple configmap
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "--set", "STANDARD=something")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	// standard should take the set value
	assert.Contains(t, string(kubectlOut), "standard=something")
	// no prompt should take the default value
	assert.Contains(t, string(kubectlOut), "noPrompt=no prompt default")
	// default should take the default value
	assert.Contains(t, string(kubectlOut), "default=a safe default")
	// variablized default should take the default value
	assert.Contains(t, string(kubectlOut), "varDefault=simple-configmap.yaml")
	// constant should take the constant value
	assert.Contains(t, string(kubectlOut), "constant=a value that does not change on package deploy")
	// nonExist should remain unset as it is not a component variable
	assert.Contains(t, string(kubectlOut), "nonExist=###ZARF_VAR_NON_EXISTENT###")

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
