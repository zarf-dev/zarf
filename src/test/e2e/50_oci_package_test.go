// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type RegistryClientTestSuite struct {
	suite.Suite
	*require.Assertions
	Reference   registry.Reference
	PackagesDir string
}

var badRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.PackagesDir = "build"

	e2e.SetupDockerRegistry(suite.T(), 555)
	suite.Reference.Registry = "localhost:555"
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	e2e.CleanFiles(local)

	e2e.TeardownRegistry(suite.T(), 555)
}

func (suite *RegistryClientTestSuite) Test_0_Publish() {
	suite.T().Log("E2E: Package Publish oci://")

	// Publish package.
	example := filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.Zarf("package", "publish", example, "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Published "+ref)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-component-actions-%s.tar.zst", e2e.Arch))
	_, stdErr, err = e2e.Zarf("package", "publish", example, "oci://"+ref, "--insecure")
	suite.Error(err, stdErr)

	// Inline publish package.
	dir := filepath.Join("examples", "helm-charts")
	stdOut, stdErr, err = e2e.Zarf("package", "create", dir, "-o", "oci://"+ref, "--insecure", "--oci-concurrency=5", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) Test_1_Pull() {
	suite.T().Log("E2E: Package Pull oci://")

	out := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-charts"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()

	// Pull the package via OCI.
	stdOut, stdErr, err := e2e.Zarf("package", "pull", "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Pulled oci://"+ref)

	// Verify the package was pulled.
	suite.FileExists(out)

	// Test pull w/ bad ref.
	stdOut, stdErr, err = e2e.Zarf("package", "pull", "oci://"+badRef.String(), "--insecure")
	suite.Error(err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) Test_2_Deploy() {
	suite.T().Log("E2E: Package Deploy oci://")

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-charts"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", "oci://"+ref, "--components=demo-helm-oci-chart", "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Remove the package via OCI.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "oci://"+ref, "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Test deploy w/ bad ref.
	_, stdErr, err = e2e.Zarf("package", "deploy", "oci://"+badRef.String(), "--insecure", "--confirm")
	suite.Error(err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_3_Inspect() {
	suite.T().Log("E2E: Package Inspect oci://")

	suite.Reference.Repository = "helm-charts"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.Zarf("package", "inspect", "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)

	// Test inspect w/ bad ref.
	_, stdErr, err = e2e.Zarf("package", "inspect", "oci://"+badRef.String(), "--insecure")
	suite.Error(err, stdErr)

	// Test inspect on a public package.
	// NOTE: This also makes sure that Zarf does not attempt auth when inspecting a public package.
	_, stdErr, err = e2e.Zarf("package", "inspect", "oci://ghcr.io/defenseunicorns/packages/dubbd-k3d:0.3.0-amd64")
	suite.NoError(err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_4_Pull_And_Deploy() {
	suite.T().Log("E2E: Package Pull oci:// && Package Deploy tarball")

	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	defer e2e.CleanFiles(local)
	// Verify the package was pulled.
	suite.FileExists(local)

	// Deploy the local package.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", local, "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) Test_5_Copy() {
	t := suite.T()
	ref := suite.Reference.String()
	dstRegistryPort := 556
	dstRef := strings.Replace(ref, fmt.Sprint(555), fmt.Sprint(dstRegistryPort), 1)

	e2e.SetupDockerRegistry(t, dstRegistryPort)
	defer e2e.TeardownRegistry(t, dstRegistryPort)

	ctx := context.TODO()

	src, err := oci.NewOrasRemote(ref)
	suite.NoError(err)
	src = src.WithInsecureConnection(true).WithContext(ctx)

	dst, err := oci.NewOrasRemote(dstRef)
	suite.NoError(err)
	dst = dst.WithInsecureConnection(true).WithContext(ctx)

	err = oci.CopyPackage(ctx, src, dst, nil, 5)
	suite.NoError(err)

	srcRoot, err := src.FetchRoot()
	suite.NoError(err)

	for _, layer := range srcRoot.Layers {
		ok, err := dst.Repo().Exists(ctx, layer)
		suite.True(ok)
		suite.NoError(err)
	}
}

func TestRegistryClientTestSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(RegistryClientTestSuite))
}
