// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestMultiPartPackage(t *testing.T) {
	t.Log("E2E: Multi-part package")

	var (
		createPath = "src/test/packages/05-multi-part"
		deployPath = fmt.Sprintf("zarf-package-multi-part-%s.tar.zst.part000", e2e.Arch)
		outputFile = "multi-part-demo.dat"
	)

	e2e.CleanFiles(deployPath, outputFile)

	// Create the package with a max size of 1MB
	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--max-package-size=1", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	parts, err := filepath.Glob("zarf-package-multi-part-*")
	require.NoError(t, err)
	// Length is 7 because there are 6 parts and 1 manifest
	require.Len(t, parts, 7)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", deployPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the package was deployed
	require.FileExists(t, outputFile)

	// deploying package combines parts back into single archive, check dir again to find all files
	parts, err = filepath.Glob("zarf-package-multi-part-*")
	e2e.CleanFiles(parts...)
	e2e.CleanFiles(outputFile)
}

func TestReproducibleTarballs(t *testing.T) {
	t.Log("E2E: Reproducible tarballs")

	var (
		createPath = filepath.Join("examples", "dos-games")
		tmp        = t.TempDir()
		tb         = filepath.Join(tmp, fmt.Sprintf("zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch))
		unpack1    = filepath.Join(tmp, "unpack1")
		unpack2    = filepath.Join(tmp, "unpack2")
	)

	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("tools", "archiver", "decompress", tb, unpack1)
	require.NoError(t, err, stdOut, stdErr)

	var pkg1 types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(unpack1, layout.ZarfYAML), &pkg1)
	require.NoError(t, err)

	b, err := os.ReadFile(filepath.Join(unpack1, layout.Checksums))
	require.NoError(t, err)
	checksums1 := string(b)

	e2e.CleanFiles(unpack1, tb)

	stdOut, stdErr, err = e2e.Zarf("package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("tools", "archiver", "decompress", tb, unpack2)
	require.NoError(t, err, stdOut, stdErr)

	var pkg2 types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(unpack2, layout.ZarfYAML), &pkg2)
	require.NoError(t, err)

	b, err = os.ReadFile(filepath.Join(unpack2, layout.Checksums))
	require.NoError(t, err)
	checksums2 := string(b)

	message.PrintDiff(checksums1, checksums2)

	require.Equal(t, pkg1.Metadata.AggregateChecksum, pkg2.Metadata.AggregateChecksum)
}
