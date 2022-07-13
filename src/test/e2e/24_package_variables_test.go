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

	// Deploy the simple configmap
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--set", "CAT=meow")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	assert.Contains(t, string(kubectlOut), "wolf=###ZARF_VAR_WOLF###")
	assert.Contains(t, string(kubectlOut), "dog=woof")
	assert.Contains(t, string(kubectlOut), "cat=meow")
	assert.Contains(t, string(kubectlOut), "fox=yuf")
	// zebra should remain unset as it is not a component variable
	assert.Contains(t, string(kubectlOut), "zebra=###ZARF_VAR_ZEBRA###")
}
