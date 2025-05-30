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
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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

	e2e.CleanFiles(t, deployPath, outputFile)

	// Create the package with a max size of 20MB
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", createPath, "--max-package-size=20", "--confirm")
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

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", deployPath, "--confirm")
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

	e2e.CleanFiles(t, parts...)
	e2e.CleanFiles(t, outputFile)
}

func TestReproducibleTarballs(t *testing.T) {
	t.Log("E2E: Reproducible tarballs")

	var (
		createPath = filepath.Join("examples", "dos-games")
		tmp        = t.TempDir()
		tb         = filepath.Join(tmp, fmt.Sprintf("zarf-package-dos-games-%s-1.2.0.tar.zst", e2e.Arch))
		unpack1    = filepath.Join(tmp, "unpack1")
		unpack2    = filepath.Join(tmp, "unpack2")
	)

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "archiver", "decompress", tb, unpack1)
	require.NoError(t, err, stdOut, stdErr)

	var pkg1 v1alpha1.ZarfPackage
	err = utils.ReadYaml(filepath.Join(unpack1, layout.ZarfYAML), &pkg1)
	require.NoError(t, err)

	e2e.CleanFiles(t, unpack1, tb)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "archiver", "decompress", tb, unpack2)
	require.NoError(t, err, stdOut, stdErr)

	var pkg2 v1alpha1.ZarfPackage
	err = utils.ReadYaml(filepath.Join(unpack2, layout.ZarfYAML), &pkg2)
	require.NoError(t, err)

	require.Equal(t, pkg1.Metadata.AggregateChecksum, pkg2.Metadata.AggregateChecksum)
}

func TestPackageTarballDirectoryStructure(t *testing.T) {
	t.Log("E2E: Package tarball directory structure")

	var (
		createPath = filepath.Join("src", "test", "packages", "05-archive-structure")
		tmp        = t.TempDir()
		tb         = filepath.Join(tmp, fmt.Sprintf("zarf-package-test-archive-structure-%s-0.0.1.tar.zst", e2e.Arch))
		unpack     = filepath.Join(tmp, "unpack")
		unpackAll  = filepath.Join(tmp, "unpack-all")
	)

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", createPath, "--confirm", "--output", tmp)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "archiver", "decompress", tb, unpack)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "archiver", "decompress", tb, unpackAll, "--unarchive-all")
	require.NoError(t, err, stdOut, stdErr)

	defer e2e.CleanFiles(t, unpack, unpackAll, tb)

	// Check the directory structure using standard decompression
	// Should be representative of the following structure:
	// |-- checksums.txt
	// |-- components
	// |   |-- test-component-1.tar
	// |   `-- test-component-2.tar
	// |-- images
	// |   |-- blobs
	// |   |   `-- sha256
	// |   |       |-- 12cba3a8e34081029e840e7ac5454c080835cbc5a7adc1620482e939283a3a49
	// |   |       |-- 4163972f9a84fde6c8db0e7d29774fd988a7668fe26c67ac09a90a61a889c92d
	// |   |       |-- 4db1b89c0bd13344176ddce2d093b9da2ae58336823ffed2009a7ea4b62d2a95
	// |   |       |-- 4f329f068415ce848969e08441627495e2a617427525d5094ddea16e133f2a0c
	// |   |       |-- 4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1
	// |   |       |-- 7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de
	// |   |       |-- 92974acd1b7d5aec7654a2df3a310f97c56b7449fc5d042ba8442dbace9a0da6
	// |   |       |-- 9c05575863777998b3fa1d45329e752631ff7a1937149b7d92fbff345f2cfa01
	// |   |       |-- b4cd0df67c961ba7f49c86c2e1e6e89d2fd1b8c40ad6fe59508db060dfac51fe
	// |   |       |-- c398742ba22c44f9bbc08dcbbdf0c978b20928fde49dceacded095bc09a46b84
	// |   |       |-- c5f44854f251e6d62481d82a0fb8bdaf0d4c89c2af149390fb4bacfbde771bd0
	// |   |       |-- d27f5493dd6e5f313c8ef5fe38f89c30124a1c9f09cc7d52477a26ec7c86dd5e
	// |   |       |-- d37d27b92cce4fb1383d5fbe32540382ea3d9662c7be3555f5a0f6a044099e1b
	// |   |       |-- d8173b5b3d825c1c19acf91cb66599f453187705ca9cdb4608d7be5482768cba
	// |   |       |-- d95fa8da986254bcd64c1251b695fe91875383dac1ed1780480fdf70f02cea3b
	// |   |       |-- d9a6c201b02b2e5c95b30592dc3e6a7b5ffaba1f20bbf4b658b73df10ea5de26
	// |   |       `-- ed0ab65044b58b192cd7cbf02b4afa1cb32b38f9e7faf02a15016e5a76cc9956
	// |   |-- index.json
	// |   |-- ingest
	// |   `-- oci-layout
	// |-- sboms.tar
	// `-- zarf.yaml

	checks := []struct {
		RelPath string
		IsDir   bool
	}{
		// top-level
		{"checksums.txt", false},
		{"components", true},
		{"images", true},
		{"sboms.tar", false},
		{"zarf.yaml", false},

		// components sub-tree
		{"components/test-component-1.tar", false},
		{"components/test-component-2.tar", false},

		// images sub-tree (non-blobs)
		{"images/index.json", false},
		{"images/ingest", true},
		{"images/oci-layout", false},
		{"images/blobs", true},
		{"images/blobs/sha256", true},
	}

	for _, c := range checks {
		p := filepath.Join(unpack, c.RelPath)
		info, err := os.Stat(p)
		require.NoError(t, err, "expected %q to exist", c.RelPath)
		if info.IsDir() != c.IsDir {
			require.Fail(t, fmt.Sprintf("expected %q IsDir=%v, but got IsDir=%v", c.RelPath, c.IsDir, info.IsDir()))
		}
	}

	// 2) Check that images/blobs/sha256 contains exactly 17 files
	blobDir := filepath.Join(unpack, "images", "blobs", "sha256")
	entries, err := os.ReadDir(blobDir)
	require.NoError(t, err, "failed to read sha256 dir %q", blobDir)

	const wantBlobs = 17
	if len(entries) != wantBlobs {
		require.Fail(t, fmt.Sprintf("expected %d entries in %q, but found %d", wantBlobs, blobDir, len(entries)))
	}

	for _, e := range entries {
		if e.IsDir() {
			require.Fail(t, fmt.Sprintf("expected blob entry %q to be a file, but it's a directory", e.Name()))
		}
	}

	// 3) Check the --unarchive-all option - performing this in-line to avoid re-creating the tarball
	// |-- checksums.txt
	// |-- components
	// |   |-- test-component-1
	// |   |   |-- charts
	// |   |   |   |-- podinfo-compose-6.4.0.tgz
	// |   |   |   `-- podinfo-compose-two-6.4.0.tgz
	// |   |   |-- data
	// |   |   |   |-- 0
	// |   |   |   |   `-- service.yaml
	// |   |   |   |       |-- coffee-ipsum.txt
	// |   |   |   |       |-- kustomization.yaml
	// |   |   |   |       |-- service.yaml
	// |   |   |   |       `-- test-values.yaml
	// |   |   |   `-- 1
	// |   |   |       `-- service.yaml
	// |   |   |           |-- coffee-ipsum.txt
	// |   |   |           |-- kustomization.yaml
	// |   |   |           |-- service.yaml
	// |   |   |           `-- test-values.yaml
	// |   |   |-- files
	// |   |   |   |-- 0
	// |   |   |   |   `-- coffee-ipsum.txt
	// |   |   |   `-- 1
	// |   |   |       `-- coffee-ipsum.txt
	// |   |   |-- manifests
	// |   |   |   |-- connect-service-0.yaml
	// |   |   |   |-- connect-service-1.yaml
	// |   |   |   |-- connect-service-two-0.yaml
	// |   |   |   |-- kustomization-connect-service-0.yaml
	// |   |   |   |-- kustomization-connect-service-1.yaml
	// |   |   |   `-- kustomization-connect-service-two-0.yaml
	// |   |   |-- repos
	// |   |   |   |-- zarf-public-test-2265377406
	// |   |   |   |   `-- README.md
	// |   |   |   `-- zarf-public-test-2395699829
	// |   |   |       `-- README.md
	// |   |   `-- values
	// |   |       |-- podinfo-compose-6.4.0-0
	// |   |       |-- podinfo-compose-6.4.0-1
	// |   |       `-- podinfo-compose-two-6.4.0-0
	// |   `-- test-component-2
	// |       |-- charts
	// |       |   |-- podinfo-compose-6.4.0.tgz
	// |       |   `-- podinfo-compose-two-6.4.0.tgz
	// |       |-- data
	// |       |   |-- 0
	// |       |   |   `-- service.yaml
	// |       |   |       |-- coffee-ipsum.txt
	// |       |   |       |-- kustomization.yaml
	// |       |   |       |-- service.yaml
	// |       |   |       `-- test-values.yaml
	// |       |   `-- 1
	// |       |       `-- service.yaml
	// |       |           |-- coffee-ipsum.txt
	// |       |           |-- kustomization.yaml
	// |       |           |-- service.yaml
	// |       |           `-- test-values.yaml
	// |       |-- files
	// |       |   |-- 0
	// |       |   |   `-- coffee-ipsum.txt
	// |       |   `-- 1
	// |       |       `-- coffee-ipsum.txt
	// |       |-- manifests
	// |       |   |-- connect-service-0.yaml
	// |       |   |-- connect-service-1.yaml
	// |       |   |-- connect-service-two-0.yaml
	// |       |   |-- kustomization-connect-service-0.yaml
	// |       |   |-- kustomization-connect-service-1.yaml
	// |       |   `-- kustomization-connect-service-two-0.yaml
	// |       |-- repos
	// |       |   |-- zarf-public-test-2265377406
	// |       |   |   `-- README.md
	// |       |   `-- zarf-public-test-2395699829
	// |       |       `-- README.md
	// |       `-- values
	// |           |-- podinfo-compose-6.4.0-0
	// |           |-- podinfo-compose-6.4.0-1
	// |           `-- podinfo-compose-two-6.4.0-0
	// |-- images
	// |   |-- blobs
	// |   |   `-- sha256
	// |   |       |-- 12cba3a8e34081029e840e7ac5454c080835cbc5a7adc1620482e939283a3a49
	// |   |       |-- 4163972f9a84fde6c8db0e7d29774fd988a7668fe26c67ac09a90a61a889c92d
	// |   |       |-- 4db1b89c0bd13344176ddce2d093b9da2ae58336823ffed2009a7ea4b62d2a95
	// |   |       |-- 4f329f068415ce848969e08441627495e2a617427525d5094ddea16e133f2a0c
	// |   |       |-- 4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1
	// |   |       |-- 7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de
	// |   |       |-- 92974acd1b7d5aec7654a2df3a310f97c56b7449fc5d042ba8442dbace9a0da6
	// |   |       |-- 9c05575863777998b3fa1d45329e752631ff7a1937149b7d92fbff345f2cfa01
	// |   |       |-- b4cd0df67c961ba7f49c86c2e1e6e89d2fd1b8c40ad6fe59508db060dfac51fe
	// |   |       |-- c398742ba22c44f9bbc08dcbbdf0c978b20928fde49dceacded095bc09a46b84
	// |   |       |-- c5f44854f251e6d62481d82a0fb8bdaf0d4c89c2af149390fb4bacfbde771bd0
	// |   |       |-- d27f5493dd6e5f313c8ef5fe38f89c30124a1c9f09cc7d52477a26ec7c86dd5e
	// |   |       |-- d37d27b92cce4fb1383d5fbe32540382ea3d9662c7be3555f5a0f6a044099e1b
	// |   |       |-- d8173b5b3d825c1c19acf91cb66599f453187705ca9cdb4608d7be5482768cba
	// |   |       |-- d95fa8da986254bcd64c1251b695fe91875383dac1ed1780480fdf70f02cea3b
	// |   |       |-- d9a6c201b02b2e5c95b30592dc3e6a7b5ffaba1f20bbf4b658b73df10ea5de26
	// |   |       `-- ed0ab65044b58b192cd7cbf02b4afa1cb32b38f9e7faf02a15016e5a76cc9956
	// |   |-- index.json
	// |   |-- ingest
	// |   `-- oci-layout
	// |-- sboms
	// |   |-- compare.html
	// |   |-- ghcr.io_stefanprodan_podinfo_6.4.0.json
	// |   |-- ghcr.io_stefanprodan_podinfo_6.4.1.json
	// |   |-- sbom-viewer-ghcr.io_stefanprodan_podinfo_6.4.0.html
	// |   |-- sbom-viewer-ghcr.io_stefanprodan_podinfo_6.4.1.html
	// |   |-- sbom-viewer-zarf-component-test-component-1.html
	// |   |-- sbom-viewer-zarf-component-test-component-2.html
	// |   |-- zarf-component-test-component-1.json
	// |   `-- zarf-component-test-component-2.json
	// `-- zarf.yaml
	comps := []string{"test-component-1", "test-component-2"}
	for _, comp := range comps {
		base := filepath.Join(unpackAll, "components", comp)

		// charts
		charts := filepath.Join(base, "charts")
		fi, err := os.Stat(charts)
		require.NoError(t, err, "[%s] charts dir error", comp)
		require.True(t, fi.IsDir(), "[%s] charts dir is not a directory", comp)
		for _, f := range []string{
			"podinfo-compose-6.4.0.tgz",
			"podinfo-compose-two-6.4.0.tgz",
		} {
			_, err := os.Stat(filepath.Join(charts, f))
			require.NoError(t, err, "[%s] missing chart %s", comp, f)
		}

		// data
		dataRoot := filepath.Join(base, "data")
		for _, idx := range []string{"0", "1"} {
			svcDir := filepath.Join(dataRoot, idx, "service.yaml")
			ents, err := os.ReadDir(svcDir)
			require.NoError(t, err, "[%s] data/%s/service.yaml error", comp, idx)
			if len(ents) != 4 {
				require.Fail(t, fmt.Sprintf("[%s] expected 4 files in data/%s/service.yaml, got %d", comp, idx, len(ents)))
			}
		}

		// files
		filesRoot := filepath.Join(base, "files")
		for _, idx := range []string{"0", "1"} {
			p := filepath.Join(filesRoot, idx, "coffee-ipsum.txt")
			fi, err := os.Stat(p)
			require.NoError(t, err, "[%s] files/%s/coffee-ipsum.txt error", comp, idx)
			if fi.IsDir() {
				require.Fail(t, fmt.Sprintf("[%s] files/%s/coffee-ipsum.txt is a directory", comp, idx))
			}
		}

		// manifests
		manDir := filepath.Join(base, "manifests")
		manEntries, err := os.ReadDir(manDir)
		require.NoError(t, err, "[%s] manifests dir error", comp)
		if len(manEntries) != 6 {
			require.Fail(t, fmt.Sprintf("[%s] expected 6 manifests, got %d", comp, len(manEntries)))
		}

		// repos
		repoDir := filepath.Join(base, "repos")
		repos, err := os.ReadDir(repoDir)
		require.NoError(t, err, "[%s] repos dir error", comp)
		if len(repos) != 2 {
			require.Fail(t, fmt.Sprintf("[%s] expected 2 repos, got %d", comp, len(repos)))
		}
		for _, r := range repos {
			readme := filepath.Join(repoDir, r.Name(), "README.md")
			fi, err := os.Stat(readme)
			require.NoError(t, err, "[%s] missing repo README %s", comp, r.Name())
			if fi.IsDir() {
				require.Fail(t, fmt.Sprintf("[%s] repo README %s is a directory", comp, r.Name()))
			}
		}

		// values
		valsDir := filepath.Join(base, "values")
		vals, err := os.ReadDir(valsDir)
		require.NoError(t, err, "[%s] values dir error", comp)
		if len(vals) != 3 {
			require.Fail(t, fmt.Sprintf("[%s] expected 3 values, got %d", comp, len(vals)))
		}
	}

	// sboms
	sbomsDir := filepath.Join(unpackAll, "sboms")
	wantFiles := []string{
		"compare.html",
		"ghcr.io_stefanprodan_podinfo_6.4.0.json",
		"ghcr.io_stefanprodan_podinfo_6.4.1.json",
		"sbom-viewer-ghcr.io_stefanprodan_podinfo_6.4.0.html",
		"sbom-viewer-ghcr.io_stefanprodan_podinfo_6.4.1.html",
		"sbom-viewer-zarf-component-test-component-1.html",
		"sbom-viewer-zarf-component-test-component-2.html",
		"zarf-component-test-component-1.json",
		"zarf-component-test-component-2.json",
	}
	entries, err = os.ReadDir(sbomsDir)
	require.NoError(t, err, "sboms dir error")
	if len(entries) != len(wantFiles) {
		require.Fail(t, fmt.Sprintf("expected %d sboms, got %d", len(wantFiles), len(entries)))
	}
	for _, fname := range wantFiles {
		fi, err := os.Stat(filepath.Join(sbomsDir, fname))
		require.NoError(t, err, "sboms/%s error", fname)
		if fi.IsDir() {
			require.Fail(t, fmt.Sprintf("sboms/%s is a directory", fname))
		}
	}
}
