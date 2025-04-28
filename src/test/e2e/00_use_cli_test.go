// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func TestUseCLI(t *testing.T) {
	t.Parallel()

	// TODO once cmd is refactored to accept an io.Writer, move this test to DevInspectDefinitionOptions.Run()
	t.Run("zarf dev inspect definition", func(t *testing.T) {
		t.Parallel()
		pathToPackage := filepath.Join("src", "test", "packages", "00-dev-inspect-definition")

		stdOut, _, err := e2e.Zarf(t, "dev", "inspect", "definition", pathToPackage, "--flavor=ice-cream", "--set=my_var=worked-as-expected", "--architecture=amd64")
		require.NoError(t, err)
		b, err := os.ReadFile(filepath.Join(pathToPackage, "expected-zarf.yaml"))
		require.NoError(t, err)
		require.Contains(t, stdOut, string(b))
	})

	t.Run("zarf dev sha256sum <local>", func(t *testing.T) {
		t.Parallel()

		// Test `zarf dev sha256sum` for a local asset
		expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
		shasumTestFilePath := "shasum-test-file"

		e2e.CleanFiles(t, shasumTestFilePath)
		t.Cleanup(func() {
			e2e.CleanFiles(t, shasumTestFilePath)
		})

		err := os.WriteFile(shasumTestFilePath, []byte("random test data 🦄\n"), helpers.ReadWriteUser)
		require.NoError(t, err)

		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "sha256sum", shasumTestFilePath)
		require.NoError(t, err, stdOut, stdErr)
		require.Equal(t, expectedShasum, stdOut, "The expected SHASUM should equal the actual SHASUM")
	})

	t.Run("zarf dev sha256sum <remote>", func(t *testing.T) {
		t.Parallel()
		expectedShasum := "a78d66b9e2b00a22edd9b4e6432a4d934621e3757f09493b12f688c7c9baca93\n"

		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "sha256sum", "https://zarf-init-resources.s3.us-east-1.amazonaws.com/injector/2025-03-24/zarf-injector-amd64")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, expectedShasum, "The expected SHASUM should equal the actual SHASUM")
	})

	t.Run("zarf package pull https", func(t *testing.T) {
		t.Parallel()
		packageShasum := "690799dbe8414238e11d4488754eee52ec264c1584cd0265e3b91e3e251e8b1a"
		packageName := "zarf-init-amd64-v0.39.0.tar.zst"
		_, _, err := e2e.Zarf(t, "package", "pull", fmt.Sprintf("https://github.com/zarf-dev/zarf/releases/download/v0.39.0/%s", packageName), "--shasum", packageShasum)
		require.NoError(t, err)
		require.FileExists(t, packageName)
		err = os.Remove(packageName)
		require.NoError(t, err)
	})

	t.Run("zarf version", func(t *testing.T) {
		t.Parallel()
		// Test `zarf version`
		version := e2e.GetZarfVersion(t)
		require.NotEmpty(t, version, "Zarf version should not be an empty string")

		// test `zarf version --output-format=json`
		stdOut, _, err := e2e.Zarf(t, "version", "--output-format=json")
		require.NoError(t, err)
		jsonVersion := fmt.Sprintf("\"version\": \"%s\"", version)
		require.Contains(t, stdOut, jsonVersion, "Zarf version should be the same in all formats")

		// test `zarf version --output-format=yaml`
		stdOut, _, err = e2e.Zarf(t, "version", "--output-format=yaml")
		require.NoError(t, err)
		yamlVersion := fmt.Sprintf("version: %s", version)
		require.Contains(t, stdOut, yamlVersion, "Zarf version should be the same in all formats")
	})

	t.Run("zarf deploy should fail when given a bad component input", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-no-components", "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)
		// Test for expected failure when given a bad component input
		packageName := fmt.Sprintf("zarf-package-no-components-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)
		_, _, err = e2e.Zarf(t, "package", "deploy", path, "--components=non-existent", "--confirm")
		require.Error(t, err)
	})

	t.Run("zarf deploy should return a warning when no components are deployed", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-no-components", "-o", tmpdir, "--confirm")
		require.NoError(t, err)
		packageName := fmt.Sprintf("zarf-package-no-components-%s.tar.zst", e2e.Arch)
		path := filepath.Join(tmpdir, packageName)

		// Test that excluding all components with a leading dash results in a warning
		_, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--components=-deselect-me", "--confirm")
		require.NoError(t, err)
		require.Contains(t, stdErr, "no components were selected for deployment")

		// Test that excluding still works even if a wildcard is given
		_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--components=*,-deselect-me", "--confirm")
		require.NoError(t, err)
		require.NotContains(t, stdErr, "DESELECT-ME COMPONENT")
	})

	t.Run("changing log level", func(t *testing.T) {
		t.Parallel()
		// Test that changing the log level actually applies the requested level
		_, stdErr, err := e2e.Zarf(t, "internal", "crc32", "zarf", "--log-level=debug")
		require.NoError(t, err)
		expectedOutString := "cfg.level=debug"
		require.Contains(t, stdErr, expectedOutString, "The log level should be changed to 'debug'")
	})

	t.Run("zarf package to test archive path", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-archive-path", "-o", tmpDir,"--flavor", runtime.GOOS,  "--confirm")
		require.NoError(t, err)

		path := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-archive-path-%s.tar.zst", e2e.Arch))
		_, _, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
		require.NoError(t, err)

		require.FileExists(t, "src/test/packages/00-archive-path/output.txt")

		b, err := os.ReadFile("src/test/packages/00-archive-path/output.txt")
		require.NoError(t, err)
		require.Equal(t, "Hello World!\n", string(b))

		e2e.CleanFiles(t, "src/test/packages/00-archive-path/output.txt")
	})

	t.Run("zarf package create with tmpdir and cache", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		cacheDir := filepath.Join(t.TempDir(), ".cache-location")
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "--zarf-cache", cacheDir, "--tmpdir", tmpdir, "--log-level=debug", "-o", tmpdir, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		files, err := os.ReadDir(filepath.Join(cacheDir, "images"))
		require.NoError(t, err, "Encountered an unexpected error when reading image cache path")
		require.Greater(t, len(files), 1)
	})

	// TODO: Refactor test as it depends on debug log output for validation.
	t.Run("zarf package deploy with tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		// run `zarf package deploy` with a specified tmp location
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-no-components", "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)
		packageName := fmt.Sprintf("zarf-package-no-components-%s.tar.zst", e2e.Arch)
		stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", filepath.Join(tmpdir, packageName), "--tmpdir", tmpdir, "--log-level=debug", "--confirm")
		require.Contains(t, stdErr, tmpdir, "The tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("remove cache", func(t *testing.T) {
		tmpdir := t.TempDir()
		// Test removal of cache
		cachePath := filepath.Join(tmpdir, ".cache-location")
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "clear-cache", "--zarf-cache", cachePath)
		require.NoError(t, err, stdOut, stdErr)
		// Check that ReadDir returns no such file or directory for the cachePath
		_, err = os.ReadDir(cachePath)
		if runtime.GOOS == "windows" {
			msg := fmt.Sprintf("open %s: The system cannot find the file specified.", cachePath)
			require.EqualError(t, err, msg, "Did not receive expected error when reading a directory that should not exist")
		} else {
			msg := fmt.Sprintf("open %s: no such file or directory", cachePath)
			require.EqualError(t, err, msg, "Did not receive expected error when reading a directory that should not exist")
		}
	})

	t.Run("gen pki", func(t *testing.T) {
		t.Parallel()
		// Test generation of PKI
		tlsCA := "tls.ca"
		tlsCert := "tls.crt"
		tlsKey := "tls.key"
		t.Cleanup(func() {
			e2e.CleanFiles(t, tlsCA, tlsCert, tlsKey)
		})
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "gen-pki", "github.com", "--sub-alt-name", "google.com")
		require.NoError(t, err, stdOut, stdErr)

		require.FileExists(t, tlsCA)

		require.FileExists(t, tlsCert)

		require.FileExists(t, tlsKey)
	})

	t.Run("zarf tools yq should function appropriately across different uses", func(t *testing.T) {
		t.Parallel()

		tmpdir := t.TempDir()
		originalPath := "src/test/packages/00-yq-checks"

		originalFile := filepath.Join(originalPath, "file1.yaml")
		originalOtherFile := filepath.Join(originalPath, "file2.yaml")

		file := filepath.Join(tmpdir, "file1.yaml")
		otherFile := filepath.Join(tmpdir, "file2.yaml")

		err := copy.Copy(originalFile, file)
		require.NoError(t, err)
		err = copy.Copy(originalOtherFile, otherFile)
		require.NoError(t, err)

		// Test that yq can eval properly
		_, stdErr, err := e2e.Zarf(t, "tools", "yq", "eval", "-i", `.items[1].name = "renamed-item"`, file)
		require.NoError(t, err, stdErr)
		stdOut, _, err := e2e.Zarf(t, "tools", "yq", ".items[1].name", file)
		require.NoError(t, err)
		require.Contains(t, stdOut, "renamed-item")

		// Test that yq ea can be used properly
		_, _, err = e2e.Zarf(t, "tools", "yq", "eval-all", "-i", `. as $doc ireduce ({}; .items += $doc.items)`, file, otherFile)
		require.NoError(t, err)
		stdOut, _, err = e2e.Zarf(t, "tools", "yq", "e", ".items | length", file)
		require.NoError(t, err)
		require.Equal(t, "4\n", stdOut)
	})
}
