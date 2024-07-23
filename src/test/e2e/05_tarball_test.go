// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

func TestMultiPartPackage(t *testing.T) {
	t.Log("E2E: Multi-part package")

	var (
		createPath = "src/test/packages/05-multi-part"
		deployPath = fmt.Sprintf("zarf-package-multi-part-%s.tar.zst.part000", e2e.Arch)
		outputFile = "multi-part-demo.dat"
	)

	e2e.CleanFiles(deployPath, outputFile)

	// Create the package with a max size of 20MB
	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--max-package-size=20", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	parts, err := filepath.Glob("zarf-package-multi-part-*")
	require.NoError(t, err)
	// Length is 4 because there are 3 parts and 1 manifest
	require.Len(t, parts, 4)
	// Check the file sizes are even
	part1FileInfo, err := os.Stat(parts[1])
	require.NoError(t, err)
	require.Equal(t, int64(20000000), part1FileInfo.Size())
	part2FileInfo, err := os.Stat(parts[2])
	require.NoError(t, err)
	require.Equal(t, int64(20000000), part2FileInfo.Size())
	// Check the package data is correct
	pkgData := types.ZarfSplitPackageData{}
	part0File, err := os.ReadFile(parts[0])
	require.NoError(t, err)
	err = json.Unmarshal(part0File, &pkgData)
	require.NoError(t, err)
	require.Equal(t, 3, pkgData.Count)
	fmt.Printf("%#v", pkgData)

	stdOut, stdErr, err = e2e.Zarf("package", "deploy", deployPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the package was deployed
	require.FileExists(t, outputFile)

	// deploying package combines parts back into single archive, check dir again to find all files
	parts, err = filepath.Glob("zarf-package-multi-part-*")
	require.NoError(t, err)
	// Length is 1 because `zarf package deploy` will recombine the file
	require.Len(t, parts, 1)
	// Ensure that the number of pkgData bytes was correct
	fullFileInfo, err := os.Stat(parts[0])
	require.NoError(t, err)
	require.Equal(t, pkgData.Bytes, fullFileInfo.Size())
	// Ensure that the pkgData shasum was correct (should be checked during deploy as well, but this is to double check)
	err = helpers.SHAsMatch(parts[0], pkgData.Sha256Sum)
	require.NoError(t, err)

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

	e2e.CleanFiles(unpack1, tb)

	stdOut, stdErr, err = e2e.Zarf("package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf("tools", "archiver", "decompress", tb, unpack2)
	require.NoError(t, err, stdOut, stdErr)

	var pkg2 types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(unpack2, layout.ZarfYAML), &pkg2)
	require.NoError(t, err)

	require.Equal(t, pkg1.Metadata.AggregateChecksum, pkg2.Metadata.AggregateChecksum)
}
