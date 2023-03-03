// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

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

var badRef = registry.Reference{
	Registry: "localhost:666",
	Repository: "zarf-test",
	Reference: "bad-tag",
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

func (suite *RegistryClientTestSuite) Test_0_Publish() {
	t := suite.T()
	t.Log("E2E: Package Publish oci://")
	// Publish package.
	example := filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.arch))
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.execZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Published: "+ref)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-dos-games-%s.tar.zst", e2e.arch))
	_, stdErr, err = e2e.execZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	require.Error(t, err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_1_Pull() {
	t := suite.T()
	t.Log("E2E: Package Pull oci://")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	out := fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.arch)
	e2e.cleanFiles(out)
	defer e2e.cleanFiles(out)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.arch)
	ref := suite.Reference.String()

	// Pull the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "pull", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled: "+ref)

	// Verify the package was pulled.
	require.FileExists(t, out)

	// Test pull w/ bad ref.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "pull", "oci://"+badRef.String(), "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled: "+ref)
}

func (suite *RegistryClientTestSuite) Test_2_Deploy() {
	t := suite.T()
	t.Log("E2E: Package Deploy oci://")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.arch)
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled: "+ref)

	// Test deploy w/ bad ref.
	_, stdErr, err = e2e.execZarfCommand("package", "deploy", "oci://"+badRef.String(), "--insecure")
	require.Error(t, err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_3_Inspect() {
	t := suite.T()
	t.Log("E2E: Package Inspect oci://")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	suite.Reference.Repository = "helm-oci-chart"
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.execZarfCommand("package", "inspect", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, suite.Reference.Repository)

	// Test inspect w/ bad ref.
	_, stdErr, err = e2e.execZarfCommand("package", "inspect", "oci://"+badRef.String(), "--insecure")
	require.Error(t, err, stdErr)
}

func TestRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryClientTestSuite))
}