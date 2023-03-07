// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type RegistryClientTestSuite struct {
	suite.Suite
	Remote         *utils.OrasRemote
	Reference      registry.Reference
	PackagesDir    string
	ZarfState      types.ZarfState
	RegistryTunnel *cluster.Tunnel
}

var badRef = registry.Reference{
	Registry:   "",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	t := suite.T()
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Get reference to the current cluster
	c, err := cluster.NewClusterWithWait(1 * time.Minute)
	require.NoError(t, err, "unable to connect to the cluster")

	// Get the Zarf state from the cluster
	state, err := c.LoadZarfState()
	require.NoError(t, err, "unable to load the current Zarf state")
	suite.ZarfState = state

	// Create a tunnel to the registry running in the cluster
	suite.RegistryTunnel, err = cluster.NewZarfTunnel()
	require.NoError(t, err, "unable to create a tunnel to the registry")
	err = suite.RegistryTunnel.Connect("registry", false)
	require.NoError(t, err, "unable to connect to the registry")
	suite.Reference.Registry = suite.RegistryTunnel.Endpoint()
	badRef.Registry = suite.RegistryTunnel.Endpoint()

	suite.PackagesDir = "build"

	_, stdErr, err := e2e.execZarfCommand("tools", "registry", "login", "--username", suite.ZarfState.RegistryInfo.PushUsername, "-p", suite.ZarfState.RegistryInfo.PushPassword, suite.Reference.Registry)
	require.NoError(t, err)
	require.Contains(t, stdErr, "logged in", "failed to login to the registry")
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	t := suite.T()
	defer e2e.teardown(t)

	suite.RegistryTunnel.Close()

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
	require.Contains(t, stdErr, "Published "+ref)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.arch))
	_, stdErr, err = e2e.execZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	require.Error(t, err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_1_Pull() {
	t := suite.T()
	t.Log("E2E: Package Pull oci://")

	out := fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.arch)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.arch)
	ref := suite.Reference.String()

	// Pull the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "pull", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled "+ref)
	defer e2e.cleanFiles("zarf-package-helm-oci-chart-amd64-0.0.1.tar.zst")

	// Verify the package was pulled.
	require.FileExists(t, out)

	// Test pull w/ bad ref.
	stdOut, stdErr, err = e2e.execZarfCommand("package", "pull", "oci://"+badRef.String(), "--insecure")
	require.Error(t, err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) Test_2_Deploy() {
	t := suite.T()
	t.Log("E2E: Package Deploy oci://")

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.arch)
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", "oci://"+ref, "--insecure", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Pulled "+ref)

	// Test deploy w/ bad ref.
	_, stdErr, err = e2e.execZarfCommand("package", "deploy", "oci://"+badRef.String(), "--insecure", "--confirm")
	require.Error(t, err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_3_Inspect() {
	t := suite.T()
	t.Log("E2E: Package Inspect oci://")

	suite.Reference.Repository = "helm-oci-chart"
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.execZarfCommand("package", "inspect", "oci://"+ref, "--insecure")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, suite.Reference.Reference)

	// Test inspect w/ bad ref.
	_, stdErr, err = e2e.execZarfCommand("package", "inspect", "oci://"+badRef.String(), "--insecure")
	require.Error(t, err, stdErr)
}

func TestRegistryClientTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryClientTestSuite))
}
