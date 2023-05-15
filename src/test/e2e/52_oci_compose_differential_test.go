// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	dconfig "github.com/docker/cli/cli/config"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

/*
HOW TO TEST:
1. Publish a standard zarf package to the registry
*/
type OCIDifferentialSuite struct {
	suite.Suite
	Remote    *utils.OrasRemote
	Reference registry.Reference
}

func (suite *OCIDifferentialSuite) SetupSuite() {

	suite.Reference.Registry = "localhost:555"

	// spin up a local registry
	registryImage := "registry:2.8.1"
	err := exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "555:5000", "--name", "registry", registryImage)
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, err := e2e.ExecZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}
	// publish one of the example packages to the registry
	examplePackagePath := filepath.Join("examples", "helm-oci-chart")
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "publish", examplePackagePath, "oci://"+suite.Reference.String(), "--insecure")
	suite.NoError(err, stdOut, stdErr)

	// build the package that we are going to publish
	anotherPackagePath := "src/test/test-packages/oci-differential"
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "create", anotherPackagePath, "--insecure", "--set=PACKAGE_VERSION=v0.24.0", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// publish the package that we just built
	packageName := "zarf-package-podinfo-with-oci-flux-amd64-v0.24.0.tar.zst"
	stdOut, stdErr, err = e2e.ExecZarfCommand("package", "publish", packageName, "oci://"+suite.Reference.String(), "--insecure")
	suite.NoError(err, stdOut, stdErr)

}

func (suite *OCIDifferentialSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
}

func (suite *OCIDifferentialSuite) Test_0_Publish_SkeletonsXXX() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")

	// fdsafdsafdsa
	anotherPackagePath := "src/test/test-packages/oci-differential"
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "create", anotherPackagePath, "--differential", "oci://"+suite.Reference.String()+"/podinfo-with-oci-flux:v0.24.0-amd64", "--insecure", "--set=PACKAGE_VERSION=v0.25.0", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Load
}

func TestOCIDifferentialSuite(t *testing.T) {
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)
	suite.Run(t, new(OCIDifferentialSuite))
}
