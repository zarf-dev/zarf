package test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempDirectoryDeploy(t *testing.T) {
	t.Log("E2E: Temporary directory deploy")

	// run `zarf package deploy` with a specified tmp location
	otherTmpPath := "/tmp/othertmp"

	e2e.setup(t)
	e2e.cleanFiles(otherTmpPath)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-component-vars-other-tmp-%s.tar.zst", e2e.arch)

	_ = os.Mkdir(otherTmpPath, 0750)

	// Deploy the simple configmap
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--tmpdir", otherTmpPath, "--log-level=debug")
	require.Contains(t, stdErr, otherTmpPath, "The other tmp path should show as being created")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	assert.Contains(t, string(kubectlOut), "dog=woof")
	assert.Contains(t, string(kubectlOut), "cat=meow")
	// zebra should remain unset as it is not a component variable
	assert.Contains(t, string(kubectlOut), "zebra=###ZARF_ZEBRA###")

	e2e.cleanFiles(otherTmpPath)
}
