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

	"github.com/stretchr/testify/require"
)

func TestUseCLI(t *testing.T) {
	t.Log("E2E: Use CLI")

	t.Run("zarf prepare sha256sum <local>", func(t *testing.T) {
		t.Parallel()

		// Test `zarf prepare sha256sum` for a local asset
		expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
		shasumTestFilePath := "shasum-test-file"

		// run `zarf package create` with a specified tmp location
		otherTmpPath := t.TempDir()

		e2e.CleanFiles(shasumTestFilePath, otherTmpPath)

		err := os.WriteFile(shasumTestFilePath, []byte("random test data 🦄\n"), 0600)
		require.NoError(t, err)

		stdOut, stdErr, err := e2e.Zarf("prepare", "sha256sum", shasumTestFilePath)
		require.NoError(t, err, stdOut, stdErr)
		require.Equal(t, expectedShasum, stdOut, "The expected SHASUM should equal the actual SHASUM")
		t.Cleanup(func() {
			e2e.CleanFiles(shasumTestFilePath)
		})
	})

	t.Run("zarf prepare sha256sum <remote>", func(t *testing.T) {
		t.Parallel()
		// Test `zarf prepare sha256sum` for a remote asset
		expectedShasum := "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"

		stdOut, stdErr, err := e2e.Zarf("prepare", "sha256sum", "https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, expectedShasum, "The expected SHASUM should equal the actual SHASUM")
	})

	t.Run("zarf version", func(t *testing.T) {
		t.Parallel()
		// Test `zarf version`
		stdOut, _, err := e2e.Zarf("version")
		require.NoError(t, err)
		require.NotEqual(t, len(stdOut), 0, "Zarf version should not be an empty string")
		require.NotEqual(t, stdOut, "UnknownVersion", "Zarf version should not be the default value")
	})

	t.Run("zarf prepare find-images", func(t *testing.T) {
		t.Parallel()
		// Test `zarf prepare find-images` for a remote asset
		stdOut, stdErr, err := e2e.Zarf("prepare", "find-images", "examples/helm-charts")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "ghcr.io/stefanprodan/podinfo:6.1.6", "The chart image should be found by Zarf")
		// Test `zarf prepare find-images` for a local asset
		require.Contains(t, stdOut, "nginx:1.16.0", "The chart image should be found by Zarf")
		// Test `zarf prepare find-images` with a chart that uses helm annotations
		stdOut, stdErr, err = e2e.Zarf("prepare", "find-images", "src/test/packages/00-helm-annotations")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "registry1.dso.mil/ironbank/opensource/kubernetes/kubectl:v1.26.4", "The kubectl image should be found by Zarf")
	})

	t.Run("zarf prepare find-images --kube-version", func(t *testing.T) {
		t.Parallel()
		// Test `zarf prepare find-images` on a chart that has a `kubeVersion` declaration greater than the default (v1.20.0)
		_, stdErr, _ := e2e.Zarf("prepare", "find-images", "src/test/packages/00-kube-version-override")
		require.Contains(t, stdErr, "Problem rendering the helm template for https://charts.jetstack.io/", "The kubeVersion declaration should prevent this from templating")

		// Test `zarf prepare find-images` with `--kube-version` specified and greater than the declared minimum (v1.21.0)
		stdOut, stdErr, err := e2e.Zarf("prepare", "find-images", "--kube-version=v1.22.0", "src/test/packages/00-kube-version-override")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "quay.io/jetstack/cert-manager-controller:v1.11.1", "The chart image should be found by Zarf")
	})

	t.Run("zarf deploy should fail when given a bad component input", func(t *testing.T) {
		t.Parallel()
		// Test for expected failure when given a bad component input
		path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
		_, _, err := e2e.Zarf("package", "deploy", path, "--components=on-create,foo,logging", "--confirm")
		require.Error(t, err)
	})

	t.Run("changing log level", func(t *testing.T) {
		t.Parallel()
		// Test that changing the log level actually applies the requested level
		_, stdErr, _ := e2e.Zarf("version", "--log-level=debug")
		expectedOutString := "Log level set to debug"
		require.Contains(t, stdErr, expectedOutString, "The log level should be changed to 'debug'")
	})

	t.Run("bad zarf package deploy w/o --insecure or --shasum", func(t *testing.T) {
		t.Parallel()
		// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
		stdOut, stdErr, err := e2e.Zarf("package", "deploy", "https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst", "--confirm")
		require.Error(t, err, stdOut, stdErr)
	})

	t.Run("zarf package create with tmpdir and cache", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		cachePath := filepath.Join(tmpdir, ".cache-location")
		stdOut, stdErr, err := e2e.Zarf("package", "create", "examples/dos-games", "--zarf-cache", cachePath, "--tmpdir", tmpdir, "--log-level=debug", "--confirm")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)

		files, err := os.ReadDir(filepath.Join(cachePath, "images"))
		require.NoError(t, err, "Encountered an unexpected error when reading image cache path")
		require.Greater(t, len(files), 1)
	})

	t.Run("zarf package inspect with tmpdir", func(t *testing.T) {
		t.Parallel()
		path := fmt.Sprintf("build/zarf-package-component-actions-%s.tar.zst", e2e.Arch)
		tmpdir := t.TempDir()
		stdOut, stdErr, err := e2e.Zarf("package", "inspect", path, "--tmpdir", tmpdir, "--log-level=debug")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("zarf package deploy with tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		// run `zarf package deploy` with a specified tmp location
		var (
			firstFile  = "first-choice-file.txt"
			secondFile = "second-choice-file.txt"
		)
		path := fmt.Sprintf("build/zarf-package-component-choice-%s.tar.zst", e2e.Arch)
		stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--tmpdir", tmpdir, "--log-level=debug", "--confirm")
		require.Contains(t, stdErr, tmpdir, "The other tmp path should show as being created")
		require.NoError(t, err, stdOut, stdErr)

		t.Cleanup(func() {
			e2e.CleanFiles(firstFile, secondFile)
		})
	})

	t.Run("remove cache", func(t *testing.T) {
		t.Parallel()
		tmpdir := t.TempDir()
		// Test removal of cache
		cachePath := filepath.Join(tmpdir, ".cache-location")
		stdOut, stdErr, err := e2e.Zarf("tools", "clear-cache", "--zarf-cache", cachePath)
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
		stdOut, stdErr, err := e2e.Zarf("tools", "gen-pki", "github.com", "--sub-alt-name", "google.com")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "Successfully created a chain of trust for github.com")

		require.FileExists(t, tlsCA)

		require.FileExists(t, tlsCert)

		require.FileExists(t, tlsKey)

		t.Cleanup(func() {
			e2e.CleanFiles(tlsCA, tlsCert, tlsKey)
		})
	})
}
