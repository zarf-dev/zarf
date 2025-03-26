// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry"
)

type PullInspectTestSuite struct {
	suite.Suite
	*require.Assertions
	Reference registry.Reference
}

var badPullInspectRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *PullInspectTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.Reference.Registry = testutil.SetupInMemoryRegistry(testutil.TestContext(suite.T()), suite.T(), 31888)
}

func (suite *PullInspectTestSuite) Test_0_Pull() {
	suite.T().Log("E2E: Package Pull oci://")

	privateKeyFlag := "--signing-key=src/test/packages/zarf-test.prv-key"
	publicKeyFlag := "--key=src/test/packages/zarf-test.pub"

	outputPath := suite.T().TempDir()
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "create", "src/test/packages/11-simple-package", "-o", outputPath, privateKeyFlag, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	out := filepath.Join(outputPath, fmt.Sprintf("zarf-package-simple-package-%s-0.0.1.tar.zst", e2e.Arch))
	ref := suite.Reference.String()
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "publish", out, "oci://"+ref, "--plain-http", publicKeyFlag)
	suite.NoError(err, stdOut, stdErr)

	simplePackageRef := fmt.Sprintf("oci://%s/simple-package:0.0.1", ref)
	// fail to pull the package without providing the public key
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", simplePackageRef, "--plain-http")
	suite.Error(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", simplePackageRef, "--plain-http", publicKeyFlag, "-o", outputPath)
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", simplePackageRef, "--plain-http", "--skip-signature-validation", "-o", outputPath)
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", "definition", simplePackageRef, "--plain-http")
	suite.Error(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", "sbom", simplePackageRef, "--plain-http", publicKeyFlag, "--output", suite.T().TempDir())
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", "oci://"+badPullInspectRef.String(), "--plain-http")
	suite.Error(err, stdOut, stdErr)
}

func (suite *PullInspectTestSuite) Test_1_Remote_Inspect() {
	suite.T().Log("E2E: Package Inspect oci://")

	// Test inspect w/ bad ref.
	_, stdErr, err := e2e.Zarf(suite.T(), "package", "inspect", "definition", "oci://"+badPullInspectRef.String(), "--plain-http")
	suite.Error(err, stdErr)

	// Test inspect on a public package.
	// NOTE: This also makes sure that Zarf does not attempt auth when inspecting a public package.
	ref := fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.2.0-%s", e2e.Arch)
	_, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", "definition", ref, "--skip-signature-validation")
	suite.NoError(err, stdErr)
}

func TestPullInspectSuite(t *testing.T) {
	suite.Run(t, new(PullInspectTestSuite))
}
