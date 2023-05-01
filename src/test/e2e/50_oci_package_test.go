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
	dconfig "github.com/docker/cli/cli/config"
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
	err := exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "5000:5000", "--name", "registry", "registry:2.8.1")
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, _, err := e2e.ExecZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}

	suite.Reference.Registry = "localhost:5000"

	suite.PackagesDir = "build"
}

func (suite *RegistryClientTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.Arch)
	e2e.CleanFiles(local)

	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "remove", "helm-oci-chart", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	_, _, _, err = exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
}

func (suite *RegistryClientTestSuite) Test_0_Publish() {
	suite.T().Log("E2E: Package Publish oci://")

	// Publish package.
	example := filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.Arch))
	ref := suite.Reference.String()
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Published "+ref)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-dos-games-%s.tar.zst", e2e.Arch))
	_, stdErr, _, err = e2e.ExecZarfCommand("package", "publish", example, "oci://"+ref, "--insecure")
	suite.Error(err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_1_Pull() {
	suite.T().Log("E2E: Package Pull oci://")

	out := fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.Arch)

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()

	// Pull the package via OCI.
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "pull", "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Pulled "+ref)

	// Verify the package was pulled.
	suite.FileExists(out)

	// Test pull w/ bad ref.
	stdOut, stdErr, _, err = e2e.ExecZarfCommand("package", "pull", "oci://"+badRef.String(), "--insecure")
	suite.Error(err, stdOut, stdErr)
}

func (suite *RegistryClientTestSuite) Test_2_Deploy() {
	suite.T().Log("E2E: Package Deploy oci://")

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "deploy", "oci://"+ref, "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Pulled "+ref)

	stdOut, stdErr, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-n=helm-oci-demo", "--no-headers")
	suite.NoError(err, stdErr)
	suite.Contains(string(stdOut), "podinfo-")

	// Test deploy w/ bad ref.
	_, stdErr, _, err = e2e.ExecZarfCommand("package", "deploy", "oci://"+badRef.String(), "--insecure", "--confirm")
	suite.Error(err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_3_Inspect() {
	suite.T().Log("E2E: Package Inspect oci://")

	suite.Reference.Repository = "helm-oci-chart"
	suite.Reference.Reference = fmt.Sprintf("0.0.1-%s", e2e.Arch)
	ref := suite.Reference.String()
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "inspect", "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Loading Zarf Package oci://"+ref)

	// Test inspect w/ bad ref.
	_, stdErr, _, err = e2e.ExecZarfCommand("package", "inspect", "oci://"+badRef.String(), "--insecure")
	suite.Error(err, stdErr)
}

func (suite *RegistryClientTestSuite) Test_4_Pull_And_Deploy() {
	suite.T().Log("E2E: Package Pull oci:// && Package Deploy tarball")

	local := fmt.Sprintf("zarf-package-helm-oci-chart-%s-0.0.1.tar.zst", e2e.Arch)
	defer e2e.CleanFiles(local)
	// Verify the package was pulled.
	suite.FileExists(local)

	// Deploy the local package.
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "deploy", local, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	stdOut, stdErr, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-n=helm-oci-demo", "--no-headers")
	suite.NoError(err, stdErr)
	suite.Contains(string(stdOut), "podinfo-")
}

func TestRegistryClientTestSuite(t *testing.T) {
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)
	suite.Run(t, new(RegistryClientTestSuite))
}
