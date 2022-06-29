package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2eComponentVariables(t *testing.T) {
	t.Log("E2E: Testing component variables")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-component-variables-%s.tar.zst", e2e.arch)

	// Deploy the simple configmap
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	assert.Contains(t, string(kubectlOut), "dog=woof")
	assert.Contains(t, string(kubectlOut), "cat=meow")
	// zebra should remain unset as it is not a component variable
	assert.Contains(t, string(kubectlOut), "zebra=###ZARF_ZEBRA###")
}
