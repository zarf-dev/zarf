package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTempDirectoryDeploy(t *testing.T) {
	t.Log("E2E: Temporary directory deploy")

	// run `zarf package deploy` with a specified tmp location
	var (
		otherTmpPath = "/tmp/othertmp"
		firstFile    = "first-choice-file.txt"
		secondFile   = "second-choice-file.txt"
	)

	e2e.setup(t)
	defer e2e.teardown(t)

	e2e.cleanFiles(otherTmpPath, firstFile, secondFile)

	path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.arch)

	_ = os.Mkdir(otherTmpPath, 0750)

	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "--tmpdir", otherTmpPath, "--log-level=debug")
	require.Contains(t, stdErr, otherTmpPath, "The other tmp path should show as being created")
	require.NoError(t, err, stdOut, stdErr)

	e2e.cleanFiles(otherTmpPath, firstFile, secondFile)
}
