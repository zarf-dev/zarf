// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry"
)

type PublishDeploySuiteTestSuite struct {
	suite.Suite
	*require.Assertions
	Reference   registry.Reference
	PackagesDir string
}

var badDeployRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *PublishDeploySuiteTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.PackagesDir = suite.T().TempDir()
	suite.Reference.Registry = testutil.SetupInMemoryRegistry(testutil.TestContext(suite.T()), suite.T(), 31889)
}

func (suite *PublishDeploySuiteTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	e2e.CleanFiles(suite.Suite.T(), local)
}

func (suite *PublishDeploySuiteTestSuite) Test_0_Publish() {
	suite.T().Log("E2E: Package Publish oci://")

	chartPackagePath := filepath.Join("examples", "helm-charts")
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "create", chartPackagePath, "-o", suite.PackagesDir)
	suite.NoError(err, stdOut, stdErr)
	// Publish package.
	example := filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))
	ref := suite.Reference.String()
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "publish", example, "oci://"+ref, "--plain-http")
	suite.NoError(err, stdOut, stdErr)

	// Pull the package via OCI.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", "oci://"+ref+"/helm-charts:0.0.1", "--plain-http")
	suite.NoError(err, stdOut, stdErr)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-component-actions-%s.tar.zst", e2e.Arch))
	_, stdErr, err = e2e.Zarf(suite.T(), "package", "publish", example, "oci://"+ref, "--plain-http")
	suite.Error(err, stdErr)

	// Inline publish package.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "create", chartPackagePath, "-o", "oci://"+ref, "--plain-http", "--oci-concurrency=5", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Inline publish flavor.
	chartPackagePath = filepath.Join("examples", "package-flavors")
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "create", chartPackagePath, "-o", "oci://"+ref, "--flavor", "oracle-cookie-crunch", "--plain-http", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Inspect published flavor.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", "definition", "oci://"+ref+"/package-flavors:1.0.0-oracle-cookie-crunch", "--plain-http")
	suite.NoError(err, stdOut, stdErr)

	// Inspect the published package.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", "definition", "oci://"+ref+"/helm-charts:0.0.1", "--plain-http")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *PublishDeploySuiteTestSuite) Test_1_Deploy() {
	suite.T().Log("E2E: Package Deploy oci://")

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-charts"
	suite.Reference.Reference = "0.0.1"
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "deploy", "oci://"+ref, "--plain-http", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Remove the package via OCI.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "remove", "oci://"+ref, "--plain-http", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Test deploy w/ bad ref.
	_, stdErr, err = e2e.Zarf(suite.T(), "package", "deploy", "oci://"+badDeployRef.String(), "--plain-http", "--confirm")
	suite.Error(err, stdErr)
}

func (suite *PublishDeploySuiteTestSuite) Test_2_Pull_And_Deploy() {
	suite.T().Log("E2E: Package Pull oci:// && Package Deploy tarball")

	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	defer e2e.CleanFiles(suite.T(), local)
	// Verify the package was pulled.
	suite.FileExists(local)

	// Deploy the local package.
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "deploy", local, "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func TestPublishDeploySuite(t *testing.T) {
	suite.Run(t, new(PublishDeploySuiteTestSuite))
}
