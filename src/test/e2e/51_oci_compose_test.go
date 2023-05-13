// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
	dconfig "github.com/docker/cli/cli/config"
	"github.com/mholt/archiver/v3"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type SkeletonSuite struct {
	suite.Suite
	Remote    *utils.OrasRemote
	Reference registry.Reference
}

var (
	importEverything   = filepath.Join("src", "test", "test-packages", "51-import-everything")
	importception      = filepath.Join("src", "test", "test-packages", "51-import-everything", "inception")
	everythingExternal = filepath.Join("src", "test", "test-packages", "everything-external")
	absNoCode          = filepath.Join("/", "tmp", "nocode")
)

func (suite *SkeletonSuite) SetupSuite() {
	err := os.MkdirAll(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts"), 0755)
	suite.NoError(err)
	err = utils.CreatePathAndCopy(filepath.Join("examples", "helm-local-chart", "chart"), filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	suite.DirExists(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))

	err = utils.CreatePathAndCopy(importEverything, everythingExternal)
	suite.NoError(err)
	suite.DirExists(everythingExternal)

	err = exec.CmdWithPrint("git", "clone", "https://github.com/kelseyhightower/nocode", absNoCode)
	suite.NoError(err)
	suite.DirExists(absNoCode)

	// TODO: (@RAZZLE) how do we want to handle this?
	image := "registry:2.8.2"

	// spin up a local registry
	err = exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "555:5000", "--name", "registry", image)
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, err := e2e.ExecZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}

	suite.Reference.Registry = "localhost:555"

	// re-add gitea
	stdOut, stdErr, err := e2e.ExecZarfCommand("init", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *SkeletonSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
	err = os.RemoveAll(everythingExternal)
	suite.NoError(err)
	err = os.RemoveAll(absNoCode)
	suite.NoError(err)
	err = os.RemoveAll(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	err = os.RemoveAll(filepath.Join("files"))
	suite.NoError(err)
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "remove", "init", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *SkeletonSuite) Test_0_Publish_Skeletons() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")

	helmLocal := filepath.Join("examples", "helm-local-chart")
	ref := suite.Reference.String()
	_, stdErr, err := e2e.ExecZarfCommand("package", "publish", helmLocal, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, stdErr, err = e2e.ExecZarfCommand("package", "publish", importEverything, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, _, err = e2e.ExecZarfCommand("package", "inspect", "oci://"+ref+"/import-everything:0.0.1-skeleton", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_1_Compose() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, _, err := e2e.ExecZarfCommand("package", "create", importEverything, "--confirm", "-o", "build", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.ExecZarfCommand("package", "create", importception, "--confirm", "-o", "build", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_3_FilePaths() {
	suite.T().Log("E2E: Skeleton Package File Paths")

	var pkg types.ZarfPackage

	unpacked := filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1", e2e.Arch))
	defer suite.NoError(os.RemoveAll(unpacked))
	pkgTar := filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch))
	err := archiver.Unarchive(pkgTar, unpacked)
	suite.NoError(err)
	suite.DirExists(unpacked)

	err = utils.ReadYaml(filepath.Join(unpacked, config.ZarfYAML), &pkg)
	suite.NoError(err)
	suite.NotNil(pkg)

	components := pkg.Components
	suite.NotNil(components)

	for idx, component := range components {
		suite.verifyComponentPaths(unpacked, component, idx)
	}

	// TODO: then unpack and repeat for importception + the skeleton package (will need to pull it first)
}

func (suite *SkeletonSuite) verifyComponentPaths(base string, component types.ZarfComponent, index int) {
	componentPath := filepath.Join(base, component.Name)
	tar := fmt.Sprintf("%s.tar", componentPath)
	err := archiver.Unarchive(tar, componentPath)
	suite.NoError(err)

	// TODO: check if cosign.pub exists if skeleton

	// TODO: now check all of the file paths
}

func TestSkeletonSuite(t *testing.T) {
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)
	suite.Run(t, new(SkeletonSuite))
}
