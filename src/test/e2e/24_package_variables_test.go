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
	_, stdErr, _ := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	expectedOutString := "variable 'CAT' must be '--set' when using the '--confirm' flag"
	require.Contains(t, stdErr, "", expectedOutString)

	// Deploy the simple configmap
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--set", "CAT=meow")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	// wolf should remain unset because it was not set during deploy
	assert.Contains(t, string(kubectlOut), "wolf=")
	// dog should take the default value
	assert.Contains(t, string(kubectlOut), "dog=woof")
	// cat should take the set value
	assert.Contains(t, string(kubectlOut), "cat=meow")
	// fox should take the created value
	assert.Contains(t, string(kubectlOut), "fox=simple-configmap.yaml")
	// dingo should take the constant value
	assert.Contains(t, string(kubectlOut), "dingo=howl")
	// zebra should remain unset as it is not a component variable
	assert.Contains(t, string(kubectlOut), "zebra=###ZARF_VAR_ZEBRA###")

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "package-variables", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
