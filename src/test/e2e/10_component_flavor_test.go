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
	flavorExamplePath string
)

func (suite *FlavorSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())

	// Setup the package paths after e2e has been initialized
	flavorExamplePath = filepath.Join("build", fmt.Sprintf("zarf-package-package-flavors-%s.tar.zst", e2e.Arch))
}

func (suite *FlavorSuite) TearDownSuite() {
	err := os.RemoveAll(flavorExamplePath)
	suite.NoError(err)
}

func (suite *FlavorSuite) Test_0_FlavorExample() {
	suite.T().Log("E2E: Package Flavor Example")

	_, stdErr, err := e2e.Zarf("package", "create", flavorExample, "-o", "build", "--flavor", "oracle-cookie-crunch", "--no-color", "--confirm")
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

func TestFlavorSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(FlavorSuite))
}
