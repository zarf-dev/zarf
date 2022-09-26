package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTemplating(t *testing.T) {
	t.Log("E2E: Temporary directory deploy")

	e2e.setup(t)
	defer e2e.teardown(t)

	// run `zarf package create` with a specified image cache location
	cachePath := "/tmp/.cache-location"
	decompressPath := "/tmp/.package-decompressed"

	e2e.cleanFiles(cachePath, decompressPath)

	pkgName := fmt.Sprintf("zarf-package-package-variables-%s.tar.zst", e2e.arch)

	// Test that not specifying a package variable results in an error
	_, stdErr, _ := e2e.execZarfCommand("package", "create", "examples/package-variables", "--confirm", "--zarf-cache", cachePath)
	expectedOutString := "variable 'CONFIG_MAP' must be '--set' when using the '--confirm' flag"
	require.Contains(t, stdErr, "", expectedOutString)

	// Test a simple package variable example
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", "examples/package-variables", "--set", "CONFIG_MAP=simple-configmap.yaml", "--set", "ACTION=template", "--confirm", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("t", "archiver", "decompress", pkgName, decompressPath)
	require.NoError(t, err, stdOut, stdErr)

	// Check that the configmap exists and is readable
	_, err = os.ReadFile(decompressPath + "/components/variable-example/manifests/simple-configmap.yaml")
	require.NoError(t, err)

	// Check variables in zarf.yaml are replaced correctly
	builtConfig, err := os.ReadFile(decompressPath + "/zarf.yaml")
	require.NoError(t, err)
	require.Contains(t, string(builtConfig), "name: FOX\n  default: simple-configmap.yaml")

	e2e.cleanFiles(cachePath, decompressPath, pkgName)
}
