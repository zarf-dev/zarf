// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/require"
)

func TestUseCLI(t *testing.T) {
	t.Log("E2E: Use CLI")

	t.Run("zarf prepare sha256sum <local>", func(t *testing.T) {
		t.Parallel()

		// Test `zarf prepare sha256sum` for a local asset
		expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
		shasumTestFilePath := "shasum-test-file"

		e2e.CleanFiles(shasumTestFilePath)
		t.Cleanup(func() {
			e2e.CleanFiles(shasumTestFilePath)
		})

		err := os.WriteFile(shasumTestFilePath, []byte("random test data 🦄\n"), helpers.ReadWriteUser)
		require.NoError(t, err)

		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "sha256sum", shasumTestFilePath)
		require.NoError(t, err, stdOut, stdErr)
		require.Equal(t, expectedShasum, stdOut, "The expected SHASUM should equal the actual SHASUM")
	})

	t.Run("zarf prepare sha256sum <remote>", func(t *testing.T) {
		t.Parallel()
		// Test `zarf prepare sha256sum` for a remote asset
		expectedShasum := "b905e647e0d7876cfd5b665632cfc43ad919dc60408f7236c5b541c53277b503\n"

		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "sha256sum", "https://zarf-init.s3.us-east-2.amazonaws.com/injector/2024-07-22/zarf-injector-arm64")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, expectedShasum, "The expected SHASUM should equal the actual SHASUM")
	})

	t.Run("zarf version", func(t *testing.T) {
		t.Parallel()
		// Test `zarf version`
		version, _, err := e2e.Zarf(t, "version")
		require.NoError(t, err)
		require.NotEmpty(t, version, "Zarf version should not be an empty string")
		version = strings.Trim(version, "\n")

		// test `zarf version --output=json`
		stdOut, _, err := e2e.Zarf(t, "version", "--output=json")
		require.NoError(t, err)
		jsonVersion := fmt.Sprintf(",\"version\":\"%s\"}", version)
		require.Contains(t, stdOut, jsonVersion, "Zarf version should be the same in all formats")

		// test `zarf version --output=yaml`
		stdOut, _, err = e2e.Zarf(t, "version", "--output=yaml")
		require.NoError(t, err)
		yamlVersion := fmt.Sprintf("version: %s", version)
		require.Contains(t, stdOut, yamlVersion, "Zarf version should be the same in all formats")
	})

	t.Run("zarf deploy should fail when given a bad component input", func(t *testing.T) {
		t.Parallel()
		// Test for expected failure when given a bad component input
		path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
		_, _, err := e2e.Zarf(t, "package", "deploy", path, "--components=on-create,foo,git-server", "--confirm")
		require.Error(t, err)
	})

	t.Run("zarf deploy should return a warning when no components are deployed", func(t *testing.T) {
		t.Parallel()
		_, _, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-no-components", "-o=build", "--confirm")
		require.NoError(t, err)
		path := fmt.Sprintf("build/zarf-package-no-components-%s.tar.zst", e2e.Arch)

		// Test that excluding all components with a leading dash results in a warning
		_, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--components=-deselect-me", "--confirm")
		require.NoError(t, err)
		require.Contains(t, stdErr, "No components were selected for deployment")

		// Test that excluding still works even if a wildcard is given
		_, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--components=*,-deselect-me", "--confirm")
		require.NoError(t, err)
		require.NotContains(t, stdErr, "DESELECT-ME COMPONENT")
	})

	t.Run("changing log level", func(t *testing.T) {
		t.Parallel()
		// Test that changing the log level actually applies the requested level
		_, stdErr, _ := e2e.Zarf(t, "internal", "crc32", "zarf", "--log-level=debug")
		expectedOutString := "Log level set to debug"
		require.Contains(t, stdErr, expectedOutString, "The log level should be changed to 'debug'")
	})

	t.Run("zarf package to test bad remote images", func(t *testing.T) {
		_, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/00-remote-pull-fail", "--confirm")
		// expecting zarf to have an error and output to stderr
		require.Error(t, err)
		// Make sure we print the get request error (only look for GET since the actual error changes based on login status)
		require.Contains(t, stdErr, "failed to find the manifest on a remote: GET")
		// And the docker error
		require.Contains(t, stdErr, "response from daemon: No such image")
	})

	t.Run("zarf package to test archive path", func(t *testing.T) {
		t.Parallel()
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "packages/distros/eks", "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		path := fmt.Sprintf("zarf-package-distro-eks-%s-0.0.3.tar.zst", e2e.Arch)
		stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		require.FileExists(t, "binaries/eksctl_Darwin_x86_64")
		require.FileExists(t, "binaries/eksctl_Darwin_arm64")
		require.FileExists(t, "binaries/eksctl_Linux_x86_64")

		e2e.CleanFiles("binaries/eksctl_Darwin_x86_64", "binaries/eksctl_Darwin_arm64", "binaries/eksctl_Linux_x86_64", path, "eks.yaml")
	})

	t.Run("zarf package create with tmpdir and cache", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		cacheDir := filepath.Join(t.TempDir(), ".cache-location")
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "--zarf-cache", cacheDir, "--tmpdir", tmpdir, "--log-level=debug", "-o=build", "--confirm")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)

		files, err := os.ReadDir(filepath.Join(cacheDir, "images"))
		require.NoError(t, err, "Encountered an unexpected error when reading image cache path")
		require.Greater(t, len(files), 1)
	})

	// TODO: Refactor test as it depends on debug log output for validation.
	t.Run("zarf package inspect with tmpdir", func(t *testing.T) {
		t.Parallel()
		path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
		tmpdir := t.TempDir()
		stdOut, stdErr, err := e2e.Zarf(t, "package", "inspect", path, "--tmpdir", tmpdir, "--log-level=debug")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)
	})

	// TODO: Refactor test as it depends on debug log output for validation.
	t.Run("zarf package deploy with tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		// run `zarf package deploy` with a specified tmp location
		var (
			firstFile  = "first-choice-file.txt"
			secondFile = "second-choice-file.txt"
		)
		t.Cleanup(func() {
			e2e.CleanFiles(firstFile, secondFile)
		})
		path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.Arch)
		stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", path, "--tmpdir", tmpdir, "--log-level=debug", "--confirm")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("remove cache", func(t *testing.T) {
		t.Parallel()
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
			e2e.CleanFiles(tlsCA, tlsCert, tlsKey)
		})
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "gen-pki", "github.com", "--sub-alt-name", "google.com")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "Successfully created a chain of trust for github.com")

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
