// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type RegistryClientTestSuite struct {
	suite.Suite
	Remote *utils.OrasRemote
	Reference registry.Reference
	PackagesDir string
	ZarfState types.ZarfState
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	t := suite.T()
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	suite.PackagesDir = "build"

	suite.Reference.Registry = "127.0.0.1:31337" // from 20_zarf_init_test.go#L31

	if err := cluster.NewClusterOrDie().SaveZarfState(suite.ZarfState); err != nil {
		t.Fatal(err)
	}

	stdOut, _, err := e2e.execZarfCommand("tools", "registry", "login", "--username", suite.ZarfState.RegistryInfo.PushUsername, "--password", suite.ZarfState.RegistryInfo.PushPassword, suite.Reference.Registry)
	require.NoError(t, err)
	require.Contains(t, stdOut, "logged in")

	suite.publishHelmOCIChart()
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	t := suite.T()
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	suite.Reference = registry.Reference{}

	// Remove the package.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "remove", "helm-oci-chart", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) publishHelmOCIChart() {
	t := suite.T()
	example := filepath.Join(suite.PackagesDir, "zarf-package-helm-oci-chart-amd64.tar.zst")
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.execZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Published: "+ref)
}

func (suite *RegistryClientTestSuite) TestPull() {
	t := suite.T()
	t.Log("E2E: Package Pull")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	out := "zarf-package-helm-oci-chart-0.0.1-amd64.tar.zst"
	e2e.cleanFiles(out)
	defer e2e.cleanFiles(out)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart" // metadata.name
	suite.Reference.Reference = "0.0.1" // metadata.version
	ref := suite.Reference.String()

	// Pull the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "pull", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled: "+ref)

	// Verify the package was pulled.
	require.FileExists(t, out)
}

func (suite *RegistryClientTestSuite) TestDeploy() {
	t := suite.T()
	t.Log("E2E: Package Deploy OCI")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart" // metadata.name
	suite.Reference.Reference = "0.0.1" // metadata.version
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled: "+ref)
}