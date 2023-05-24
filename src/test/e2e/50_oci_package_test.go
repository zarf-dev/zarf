// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type RegistryClientTestSuite struct {
	suite.Suite
	Remote      *utils.OrasRemote
	Reference   registry.Reference
	PackagesDir string
}

var badRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *RegistryClientTestSuite) SetupSuite() {
	// spin up a local registry
	e2e.SetupDockerRegistry(suite.T(), 5000)
	suite.Reference.Registry = "localhost:5000"
	suite.PackagesDir = "build"
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	e2e.CleanFiles(local)

	stdOut, stdErr, err := e2e.Zarf("package", "remove", "helm-charts", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	_, _, err = exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
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
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch))
	_, stdErr, err = e2e.Zarf("package", "publish", example, "oci://"+ref, "--insecure")
	suite.Error(err, stdErr)
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
	suite.Contains(stdErr, "Pulled "+ref)

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
	suite.Contains(stdErr, "Pulled "+ref)

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
	suite.Contains(stdErr, "without downloading the entire package.")

	// Test inspect w/ bad ref.
	_, stdErr, err = e2e.Zarf("package", "inspect", "oci://"+badRef.String(), "--insecure")
	suite.Error(err, stdErr)
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

func TestRegistryClientTestSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(RegistryClientTestSuite))
}
