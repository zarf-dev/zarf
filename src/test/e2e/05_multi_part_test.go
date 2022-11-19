package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiPartPackage(t *testing.T) {
	t.Log("E2E: Multi-part package")

	var (
		createPath = "examples/multi-part/"
		deployPath = fmt.Sprintf("zarf-package-multi-part-%s-tar.zst.part000", e2e.arch)
		outputFile = "multi-part-demo.dat"
	)

	e2e.setup(t)
	defer e2e.teardown(t)

	e2e.cleanFiles(deployPath, outputFile)

	// Create the package with a max size of 1MB
	stdOut, stdErr, err := e2e.execZarfCommand("package", "create", createPath, "--confirm", "--max-package-size=1")
	require.NoError(t, err, stdOut, stdErr)

	list, err := filepath.Glob("zarf-package-multi-part-*")
	require.NoError(t, err)
	require.Len(t, list, 6)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", deployPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the package was deployed
	require.FileExists(t, outputFile)

	e2e.cleanFiles(deployPath, outputFile)
}
