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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUseCLI(t *testing.T) {
	t.Log("E2E: Use CLI")
	e2e.Setup(t)
	defer e2e.Teardown(t)

	// Test `zarf prepare sha256sum` for a local asset
	expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
	shasumTestFilePath := "shasum-test-file"

	// run `zarf package create` with a specified image cache location
	cachePath := filepath.Join(os.TempDir(), ".cache-location")
	imageCachePath := filepath.Join(cachePath, "images")

	// run `zarf package create` with a specified tmp location
	otherTmpPath := filepath.Join(os.TempDir(), "othertmp")

	e2e.CleanFiles(shasumTestFilePath, cachePath, otherTmpPath)

	err := os.WriteFile(shasumTestFilePath, []byte("random test data 🦄\n"), 0600)
	assert.NoError(t, err)

	stdOut, stdErr, err := e2e.ExecZarfCommand("prepare", "sha256sum", shasumTestFilePath)
	assert.NoError(t, err, stdOut, stdErr)
	assert.Equal(t, expectedShasum, stdOut, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf prepare sha256sum` for a remote asset
	expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"

	stdOut, stdErr, err = e2e.ExecZarfCommand("prepare", "sha256sum", "https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt")
	assert.NoError(t, err, stdOut, stdErr)
	assert.Contains(t, stdOut, expectedShasum, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf version`
	stdOut, _, err = e2e.ExecZarfCommand("version")
	assert.NoError(t, err)
	assert.NotEqual(t, len(stdOut), 0, "Zarf version should not be an empty string")
	assert.NotEqual(t, stdOut, "UnknownVersion", "Zarf version should not be the default value")

	// Test `zarf prepare find-images` for a remote asset
	stdOut, stdErr, err = e2e.ExecZarfCommand("prepare", "find-images", "examples/helm-alt-release-name")
	assert.NoError(t, err, stdOut, stdErr)
	assert.Contains(t, stdOut, "ghcr.io/stefanprodan/podinfo:6.1.6", "The chart image should be found by Zarf")

	// Test `zarf prepare find-images` for a local asset
	stdOut, stdErr, err = e2e.ExecZarfCommand("prepare", "find-images", "examples/helm-local-chart")
	assert.NoError(t, err, stdOut, stdErr)
	assert.Contains(t, stdOut, "nginx:1.16.0", "The chart image should be found by Zarf")

	// Test `zarf prepare find-images` on a chart that has a `kubeVersion` declaration greater than the default (v1.20.0)
	_, stdErr, err = e2e.ExecZarfCommand("prepare", "find-images", "src/test/test-packages/00-kube-version-override")
	require.Contains(t, stdErr, "Problem rendering the helm template for https://charts.jetstack.io/", "The kubeVersion declaration should prevent this from templating")

	// Test `zarf prepare find-images` with `--kube-version` specified and greater than the declared minimum (v1.21.0)
	stdOut, stdErr, err = e2e.ExecZarfCommand("prepare", "find-images", "--kube-version=v1.22.0", "src/test/test-packages/00-kube-version-override")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "quay.io/jetstack/cert-manager-controller:v1.11.1", "The chart image should be found by Zarf")

	// Test for expected failure when given a bad component input
	_, _, err = e2e.ExecZarfCommand("init", "--confirm", "--components=k3s,foo,logging")
	assert.Error(t, err)

	// Test that changing the log level actually applies the requested level
	_, stdErr, _ = e2e.ExecZarfCommand("version", "--log-level=debug")
	expectedOutString := "Log level set to debug"
	require.Contains(t, stdErr, expectedOutString, "The log level should be changed to 'debug'")

	// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "deploy", "https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst", "--confirm")
	assert.Error(t, err, stdOut, stdErr)

	pkgName := fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch)

	_ = os.Mkdir(otherTmpPath, 0750)
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "create", "examples/dos-games", "--confirm", "--zarf-cache", cachePath, "--tmpdir", otherTmpPath, "--log-level=debug")
	require.Contains(t, stdErr, otherTmpPath, "The other tmp path should show as being created")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "inspect", pkgName, "--tmpdir", otherTmpPath, "--log-level=debug")
	require.Contains(t, stdErr, otherTmpPath, "The other tmp path should show as being created")
	require.NoError(t, err, stdOut, stdErr)

	e2e.CleanFiles(pkgName)

	files, err := os.ReadDir(imageCachePath)
	require.NoError(t, err, "Encountered an unexpected error when reading image cache path")
	assert.Greater(t, len(files), 1)

	// Test removal of cache
	stdOut, stdErr, err = e2e.ExecZarfCommand("tools", "clear-cache", "--zarf-cache", cachePath)
	require.NoError(t, err, stdOut, stdErr)

	// Check that ReadDir returns no such file or directory for the cachePath
	_, err = os.ReadDir(cachePath)
	if runtime.GOOS == "windows" {
		msg := fmt.Sprintf("open %s: The system cannot find the file specified.", cachePath)
		assert.EqualError(t, err, msg, "Did not receive expected error when reading a directory that should not exist")
	} else {
		msg := fmt.Sprintf("open %s: no such file or directory", cachePath)
		assert.EqualError(t, err, msg, "Did not receive expected error when reading a directory that should not exist")
	}

	// Test generation of PKI
	tlsCA := "tls.ca"
	tlsCert := "tls.crt"
	tlsKey := "tls.key"
	stdOut, stdErr, err = e2e.ExecZarfCommand("tools", "gen-pki", "github.com", "--sub-alt-name", "google.com")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Successfully created a chain of trust for github.com")

	_, err = os.ReadFile(tlsCA)
	require.NoError(t, err)

	_, err = os.ReadFile(tlsCert)
	require.NoError(t, err)

	_, err = os.ReadFile(tlsKey)
	require.NoError(t, err)

	e2e.CleanFiles(shasumTestFilePath, cachePath, otherTmpPath, pkgName, tlsCA, tlsCert, tlsKey)
}
