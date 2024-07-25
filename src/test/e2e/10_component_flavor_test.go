// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type FlavorSuite struct {
	suite.Suite
	*require.Assertions
}

var (
	flavorExample     = filepath.Join("examples", "package-flavors")
	flavorTest        = filepath.Join("src", "test", "packages", "10-package-flavors")
	flavorExamplePath string
	flavorTestAMDPath = filepath.Join("build", "zarf-package-test-package-flavors-amd64.tar.zst")
	flavorTestARMPath = filepath.Join("build", "zarf-package-test-package-flavors-arm64.tar.zst")
)

func (suite *FlavorSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())

	// Setup the example package path after e2e has been initialized
	flavorExamplePath = filepath.Join("build", fmt.Sprintf("zarf-package-package-flavors-%s-1.0.0.tar.zst", e2e.Arch))
}

func (suite *FlavorSuite) TearDownSuite() {
	err := os.RemoveAll(flavorExamplePath)
	suite.NoError(err)
	err = os.RemoveAll(flavorTestAMDPath)
	suite.NoError(err)
	err = os.RemoveAll(flavorTestARMPath)
	suite.NoError(err)
}

func (suite *FlavorSuite) Test_0_FlavorExample() {
	suite.T().Log("E2E: Package Flavor Example")

	_, stdErr, err := e2e.Zarf(suite.T(), "package", "create", flavorExample, "-o", "build", "--flavor", "oracle-cookie-crunch", "--no-color", "--confirm")
	suite.NoError(err)

	// Ensure that the oracle image is included
	suite.Contains(stdErr, `oraclelinux:9-slim`)

	// Ensure that the common pod was included
	suite.Contains(stdErr, `description: The pod that runs the specified flavor of Enterprise Linux`)

	// Ensure that the other flavors are not included
	suite.NotContains(stdErr, `rockylinux:9-minimal`)
	suite.NotContains(stdErr, `almalinux:9-minimal`)
	suite.NotContains(stdErr, `opensuse/leap:15`)
}

func (suite *FlavorSuite) Test_1_FlavorArchFiltering() {
	suite.T().Log("E2E: Package Flavor + Arch Filtering")

	_, stdErr, err := e2e.Zarf(suite.T(), "package", "create", flavorTest, "-o", "build", "--flavor", "vanilla", "-a", "amd64", "--no-color", "--confirm")
	suite.NoError(err)

	// Ensure that the initial filter was applied
	suite.Contains(stdErr, `
- name: combined
  description: vanilla-amd`)

	// Ensure that the import filter was applied
	suite.Contains(stdErr, `
- name: via-import
  description: vanilla-amd`)

	// Ensure that the other flavors / architectures are not included
	suite.NotContains(stdErr, `vanilla-arm`)
	suite.NotContains(stdErr, `chocolate-amd`)
	suite.NotContains(stdErr, `chocolate-arm`)

	_, stdErr, err = e2e.Zarf(suite.T(), "package", "create", flavorTest, "-o", "build", "--flavor", "chocolate", "-a", "amd64", "--no-color", "--confirm")
	suite.NoError(err)

	// Ensure that the initial filter was applied
	suite.Contains(stdErr, `
- name: combined
  description: chocolate-amd`)

	// Ensure that the import filter was applied
	suite.Contains(stdErr, `
- name: via-import
  description: chocolate-amd`)

	// Ensure that the other flavors / architectures are not included
	suite.NotContains(stdErr, `vanilla-arm`)
	suite.NotContains(stdErr, `vanilla-amd`)
	suite.NotContains(stdErr, `chocolate-arm`)

	_, stdErr, err = e2e.Zarf(suite.T(), "package", "create", flavorTest, "-o", "build", "--flavor", "chocolate", "-a", "arm64", "--no-color", "--confirm")
	suite.NoError(err)

	// Ensure that the initial filter was applied
	suite.Contains(stdErr, `
- name: combined
  description: chocolate-arm`)

	// Ensure that the import filter was applied
	suite.Contains(stdErr, `
- name: via-import
  description: chocolate-arm`)

	// Ensure that the other flavors / architectures are not included
	suite.NotContains(stdErr, `vanilla-arm`)
	suite.NotContains(stdErr, `vanilla-amd`)
	suite.NotContains(stdErr, `chocolate-amd`)
}

func TestFlavorSuite(t *testing.T) {
	suite.Run(t, new(FlavorSuite))
}
