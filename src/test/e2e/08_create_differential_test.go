// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	"github.com/stretchr/testify/require"
)

// TestCreateDifferential creates several differential packages and ensures the already built images and repos and not included in the new package
func TestCreateDifferential(t *testing.T) {
	t.Log("E2E: Test Differential Package Behavior")
	tmpPath, _ := utils.MakeTempDir("")

	packagePath := "src/test/test-packages/08-differential-package"
	packageName := "zarf-package-differential-package-amd64-v0.25.0.tar.zst"
	differentialPackageName := "zarf-package-differential-package-amd64-v0.25.0-differential-v0.26.0.tar.zst"
	differentialFlag := fmt.Sprintf("--differential=%s", packageName)

	// Build the package a first time
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "create", packagePath, "--set=PACKAGE_VERSION=v0.25.0", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(packageName)

	// Build the differential package without changing the version
	_, stdErr, err = e2e.ExecZarfCommand("package", "create", packagePath, "--set=PACKAGE_VERSION=v0.25.0", differentialFlag, "--confirm")
	require.Error(t, err, "zarf package create should have errored when a differential package was being created without updating the package version number")
	require.Contains(t, stdErr, "unable to create a differential package with the same version")

	// Build the differential package
	_, stdErr, err = e2e.ExecZarfCommand("package", "create", packagePath, "--set=PACKAGE_VERSION=v0.26.0", differentialFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(differentialPackageName)

	// Extract the yaml of the differential package
	err = archiver.Extract(differentialPackageName, "zarf.yaml", tmpPath)
	require.NoError(t, err, "unable to extract zarf.yaml from the differential git package")

	// Load the extracted zarf.yaml specification
	var differentialZarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(tmpPath, "zarf.yaml"), &differentialZarfConfig)
	require.NoError(t, err, "unable to read zarf.yaml from the differential git package")

	// Get a list of all images and repos that are inside of the differential package
	actualGitRepos := []string{}
	actualImages := []string{}
	for _, component := range differentialZarfConfig.Components {
		actualGitRepos = append(actualGitRepos, component.Repos...)
		actualImages = append(actualImages, component.Images...)
	}

	/* Validate we have ONLY the git repos we expect to have */
	expectedGitRepos := []string{
		"https://github.com/stefanprodan/podinfo.git",
		"https://github.com/kelseyhightower/nocode.git",
		"https://github.com/DoD-Platform-One/big-bang.git@refs/heads/release-1.54.x",
		"https://github.com/defenseunicorns/zarf.git@refs/tags/v0.26.0",
	}
	require.Len(t, actualGitRepos, 4, "zarf.yaml from the differential package does not contain the correct number of repos")
	for _, expectedRepo := range expectedGitRepos {
		require.Contains(t, actualGitRepos, expectedRepo, fmt.Sprintf("unable to find expected repo %s", expectedRepo))
	}

	/* Validate we have ONLY the images we expect to have */
	expectedImages := []string{
		"ghcr.io/stefanprodan/podinfo:latest",
		"ghcr.io/defenseunicorns/zarf/agent:v0.26.0",
	}
	require.Len(t, actualImages, 2, "zarf.yaml from the differential package does not contain the correct number of images")
	for _, expectedImage := range expectedImages {
		require.Contains(t, actualImages, expectedImage, fmt.Sprintf("unable to find expected image %s", expectedImage))
	}

	e2e.CleanFiles(tmpPath)
}
