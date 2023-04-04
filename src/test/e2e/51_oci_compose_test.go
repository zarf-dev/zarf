// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	dconfig "github.com/docker/cli/cli/config"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type SkeletonSuite struct {
	suite.Suite
	Remote    *utils.OrasRemote
	Reference registry.Reference
}

func (suite *SkeletonSuite) SetupSuite() {
	image := fmt.Sprintf("%s:%s", config.ZarfSeedImage, config.ZarfSeedTag)

	// spin up a local registry
	err := exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "5000:5000", "--name", "registry", image)
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, err := e2e.execZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}

	suite.Reference.Registry = "localhost:5000"
}

func (suite *SkeletonSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_0_Publish() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")

	// Publish skeleton package.
	noWaitExample := filepath.Join("examples", "helm-no-wait")
	ref := suite.Reference.String()
	_, stdErr, err := e2e.execZarfCommand("package", "publish", noWaitExample, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)
}

func (suite *SkeletonSuite) Test_1_Compose() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	// Compose skeleton package of import-everything.
	ref := suite.Reference.String()
	importEverythingExample := filepath.Join("examples", "import-everything")
	_, stdErr, err := e2e.execZarfCommand("package", "create", importEverythingExample, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)
}

// func (suite *SkeletonSuite) Test_2_Deploy() {
// func (suite *SkeletonSuite) Test_3_BadImports() {
// func (suite *SkeletonSuite) Test_2_Deploy() {

func TestSkeltonSuite(t *testing.T) {
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)
	suite.Run(t, new(SkeletonSuite))
}

// to test:
// publish skeleton package
// compose skeleton package
// deploy newly created package
// publish newly created package
