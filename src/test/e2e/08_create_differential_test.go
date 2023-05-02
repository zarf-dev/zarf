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

	tmpPath, _ := utils.MakeTempDir("")

	t.Log("E2E: Test Differential Package Behavior")
	testDifferentialGitRepos(t, tmpPath)
	testDifferentialImages(t, tmpPath)

	e2e.CleanFiles(tmpPath)
}

// verify that any repo that isn't a specific commit hash or a tag is still included with the differential package
func testDifferentialGitRepos(t *testing.T, tmpPath string) {
	originalGitPackagePath := fmt.Sprintf("build/zarf-package-git-data-%s-v1.0.0.tar.zst", e2e.Arch)
	gitDiffPackagePath := "examples/git-data"
	gitDifferentialFlag := fmt.Sprintf("--differential=%s", originalGitPackagePath)
	gitDifferentialPackageName := "zarf-package-git-data-amd64-v1.0.0-differential-v1.0.0.tar.zst"

	// Build the differential packages
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "create", gitDiffPackagePath, gitDifferentialFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "You are creating a differential package with the same version as the package you are using")
	// Extract Git Zarf YAML
	err = archiver.Extract(gitDifferentialPackageName, "zarf.yaml", tmpPath)
	require.NoError(t, err, "unable to extract zarf.yaml from the differential git package")

	var differentialZarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(tmpPath, "zarf.yaml"), &differentialZarfConfig)
	require.NoError(t, err, "unable to read zarf.yaml from the differential git package")

	expectedGitRepos := []string{
		"https://github.com/stefanprodan/podinfo.git",
		"https://github.com/kelseyhightower/nocode.git",
		"https://github.com/DoD-Platform-One/big-bang.git@refs/heads/release-1.54.x",
	}
	actualGitRepos := []string{}
	for _, component := range differentialZarfConfig.Components {
		actualGitRepos = append(actualGitRepos, component.Repos...)
	}

	// Ensure all the repos we expect to be there are there
	require.Len(t, actualGitRepos, 3, "zarf.yaml from the differential git package does not contain the correct number of repos")
	for _, expectedRepo := range expectedGitRepos {
		require.Contains(t, actualGitRepos, expectedRepo, fmt.Sprintf("unable to find expected repo %s", expectedRepo))
	}

	e2e.CleanFiles(filepath.Join(tmpPath, "zarf.yaml"), gitDifferentialPackageName)
}

// verify that images that are tagged are removed from the differential package
func testDifferentialImages(t *testing.T, tmpPath string) {
	originalImagePackagePath := fmt.Sprintf("build/zarf-package-flux-test-%s.tar.zst", e2e.Arch) // @JPERRY double check the version here too
	imageDiffPackagePath := "examples/flux-test"
	imageDifferentialFlag := fmt.Sprintf("--differential=%s", originalImagePackagePath)
	imageDifferentialPackageName := "zarf-package-flux-test-amd64-differential.tar.zst"

	// Build the differential package
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "create", imageDiffPackagePath, imageDifferentialFlag, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "You are creating a differential package with the same version as the package you are using")

	// Extract the zarf.yaml of the new package
	err = archiver.Extract(imageDifferentialPackageName, "zarf.yaml", tmpPath)
	require.NoError(t, err, "unable to extract zarf.yaml from the differential image package")

	var differentialZarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(tmpPath, "zarf.yaml"), &differentialZarfConfig)
	require.NoError(t, err, "unable to read zarf.yaml from the differential image package")

	actualImages := []string{}
	for _, component := range differentialZarfConfig.Components {
		actualImages = append(actualImages, component.Images...)
	}

	require.Len(t, actualImages, 0, "zarf.yaml from the differential image package does not contain the correct number of images")
	e2e.CleanFiles(filepath.Join(tmpPath, "zarf.yaml"), imageDifferentialPackageName)
}
