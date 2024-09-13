// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type PullInspectTestSuite struct {
	suite.Suite
	*require.Assertions
	Reference   registry.Reference
	PackagesDir string
}

var badPullInspectRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *PullInspectTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.PackagesDir = "build"
}

func (suite *PullInspectTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch)
	e2e.CleanFiles(local)
}

func (suite *PullInspectTestSuite) Test_0_Pull() {
	suite.T().Log("E2E: Package Pull oci://")

	out := fmt.Sprintf("zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch)

	// Build the fully qualified reference.
	ref := fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.0.0-%s", e2e.Arch)

	// Pull the package via OCI.
	stdOut, stdErr, err := e2e.Zarf(suite.T(), "package", "pull", ref)
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, fmt.Sprintf("Pulling %q", ref))
	suite.Contains(stdErr, "Validating full package checksums")
	suite.NotContains(stdErr, "Package signature validated!")

	sbomTmp := suite.T().TempDir()

	// Verify the package was pulled correctly.
	suite.FileExists(out)
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", out, "--key", "https://raw.githubusercontent.com/zarf-dev/zarf/v0.38.2/cosign.pub", "--sbom-out", sbomTmp)
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Validating SBOM checksums")
	suite.Contains(stdErr, "Package signature validated!")

	// Test pull w/ bad ref.
	stdOut, stdErr, err = e2e.Zarf(suite.T(), "package", "pull", "oci://"+badPullInspectRef.String(), "--plain-http")
	suite.Error(err, stdOut, stdErr)
}

func (suite *PullInspectTestSuite) Test_1_Remote_Inspect() {
	suite.T().Log("E2E: Package Inspect oci://")

	// Test inspect w/ bad ref.
	_, stdErr, err := e2e.Zarf(suite.T(), "package", "inspect", "oci://"+badPullInspectRef.String(), "--plain-http")
	suite.Error(err, stdErr)

	// Test inspect on a public package.
	// NOTE: This also makes sure that Zarf does not attempt auth when inspecting a public package.
	ref := fmt.Sprintf("oci://ghcr.io/zarf-dev/packages/dos-games:1.0.0-%s", e2e.Arch)
	_, stdErr, err = e2e.Zarf(suite.T(), "package", "inspect", ref)
	suite.NoError(err, stdErr)
}

func TestPullInspectSuite(t *testing.T) {
	suite.Run(t, new(PullInspectTestSuite))
}
