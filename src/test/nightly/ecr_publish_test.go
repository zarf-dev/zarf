// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

var (
	e2e test.ZarfE2ETest //nolint:gochecknoglobals
)

// TestECRPublishing tests pushing, inspecting, and pulling signed Zarf packages to/from ECR.
/*
This test assumes the following:
  1. The user running the test has a current valid credential to the public.ecr.aws/t8y5r5z5/zarf-nightly repository in their docker config.json
  2. The zarf CLI has been built and is available in the build directory
*/
func TestECRPublishing(t *testing.T) {
	t.Log("E2E: Testing component actions")

	// Work from the root directory of the project
	os.Chdir("../../../")

	// Create a tmpDir for us to use during this test
	tmpDir := os.TempDir()
	defer e2e.CleanFiles(tmpDir)

	// Set up the e2e configs
	e2e.Arch = config.GetArch()
	e2e.ZarfBinPath = path.Join("build", test.GetCLIName())
	e2e.ApplianceMode = true
	e2e.RunClusterTests = false

	// Set up variables for common names/locations
	testPackageName := "helm-oci-chart"
	testPackageVersion := "0.0.1"
	testPackageFileName := fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", testPackageName, e2e.Arch, testPackageVersion)
	testPackageLocation := filepath.Join(tmpDir, testPackageFileName)
	registryURL := "oci://public.ecr.aws/t8y5r5z5/zarf-nightly"
	upstreamPackageURL := fmt.Sprintf("%s/%s:%s-%s", registryURL, testPackageName, testPackageVersion, e2e.Arch)
	keyFlag := fmt.Sprintf("--key=%s", "./src/test/test-packages/zarf-test.pub")

	// Build the package with our test signature
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "create", "examples/helm-oci-chart", "--key=./src/test/test-packages/zarf-test.prv-key", "--confirm", fmt.Sprintf("-o=%s", tmpDir))
	require.NoError(t, err, stdOut, stdErr)
	require.FileExists(t, testPackageLocation)

	// Validate that we can publish the package to ECR without an issue
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "publish", testPackageLocation, registryURL, "-l=trace")
	require.NoError(t, err, stdOut, stdErr)

	// Ensure we get a warning when trying to inspect the online published package
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "inspect", upstreamPackageURL, keyFlag, "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Zarf is unable to validate the checksums of remote OCI packages.")
	require.Contains(t, stdErr, "Package signature validated!")

	// Ensure we get an error when trying to pull the package without providing the public key
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "pull", upstreamPackageURL, "-l=trace")
	require.Error(t, err, stdOut, stdErr) //TODO: look for a specific error instead of just allowing ANY error

	// Validate that we can pull the package down from ECR
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "pull", upstreamPackageURL, keyFlag, "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(testPackageFileName)

	// Ensure we get a warning when trying to inspect the package without providing the public key
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "inspect", testPackageFileName, "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "The package you are inspecting has been signed but a public key was not provided.")
	require.Contains(t, stdErr, "All of the checksums matched!")

	// Validate that we get no warnings when inspecting the package while providing the public key
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "inspect", testPackageFileName, keyFlag, "-l=trace")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "All of the checksums matched!")
	require.Contains(t, stdErr, "Package signature validated!")
}
