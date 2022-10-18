package test

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestArchiver(t *testing.T) {
	defer e2e.teardown(t)
	var (
		pathToArchive  = "examples/terraform/"
		archivePath    = "examples/terraform/tmp.tar.gz"
		unarchivedPath = "examples/terraform/tmp"
	)

	e2e.cleanFiles(archivePath, unarchivedPath)

	testArchiverOverwrite(t, pathToArchive, archivePath, unarchivedPath)

	e2e.cleanFiles(archivePath, unarchivedPath)
}

func testArchiverOverwrite(t *testing.T, pathToArchive string, archivePath string, unarchivedPath string) {
	stdOut, _, err := e2e.execZarfCommand("tools", "archiver", "compress", pathToArchive, archivePath)
	require.NoError(t, err)

	_, _, err = e2e.execZarfCommand("tools", "archiver", "decompress", archivePath, unarchivedPath)
	require.NoError(t, err)

	// test helpful error message is displayed if no overwrite flag
	stdOut, _, err = e2e.execZarfCommand("tools", "archiver", "decompress", archivePath, unarchivedPath)
	helpfulErrorMsg := fmt.Sprintf("Cannot decompress %s because %s already exists (use --overwrite flag to overwrite existing folder)", archivePath, unarchivedPath)
	require.Contains(t, stdOut, helpfulErrorMsg)

	// test overwrite flag
	//stdOut, _, err := e2e.execZarfCommand("tools", "archiver", "decompress", "./tmp", "--overwrite")

}
