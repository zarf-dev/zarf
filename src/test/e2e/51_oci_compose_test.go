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

type SkeletonSuite struct {
	suite.Suite
	Remote    *utils.OrasRemote
	Reference registry.Reference
}

// go test ./src/test/e2e/... -v -run TestSkeletonSuite -count=1 -failfast

var (
	importEverything   = filepath.Join("src", "test", "test-packages", "51-import-everything")
	everythingExternal = filepath.Join("src", "test", "test-packages", "everything-external")
	absEverything      = filepath.Join("/", "tmp", "everything-external")
	absDosGames        = filepath.Join("/", "tmp", "dos-games")
	absNoCode          = filepath.Join("/", "tmp", "nocode")
)

func (suite *SkeletonSuite) SetupSuite() {
	err := exec.CmdWithPrint("cp", "-r", filepath.Join("examples", "helm-local-chart", "chart"), filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	suite.DirExists(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))

	err = exec.CmdWithPrint("cp", filepath.Join("examples", "data-injection", "manifest.yaml"), filepath.Join("src", "test", "test-packages", "51-import-everything", "manifests", "data-injection.yaml"))
	suite.NoError(err)
	suite.FileExists(filepath.Join("src", "test", "test-packages", "51-import-everything", "manifests", "data-injection.yaml"))

	err = exec.CmdWithPrint("cp", "-r", importEverything, everythingExternal)
	suite.NoError(err)
	suite.DirExists(everythingExternal)

	err = exec.CmdWithPrint("cp", "-r", importEverything, absEverything)
	suite.NoError(err)
	suite.DirExists(absEverything)

	err = exec.CmdWithPrint("cp", "-r", filepath.Join("examples", "dos-games"), absDosGames)
	suite.NoError(err)
	suite.DirExists(absDosGames)

	err = exec.CmdWithPrint("git", "clone", "https://github.com/kelseyhightower/nocode", absNoCode)
	suite.NoError(err)
	suite.DirExists(absNoCode)

	image := "registry:2.8.1"

	// spin up a local registry
	err = exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "666:5000", "--name", "registry", image)
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, err := e2e.execZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}

	suite.Reference.Registry = "localhost:666"
}

func (suite *SkeletonSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", everythingExternal)
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", absEverything)
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", absDosGames)
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", absNoCode)
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", "files")
	suite.NoError(err)
	err = exec.CmdWithPrint("rm", "-rf", filepath.Join("src", "test", "test-packages", "51-import-everything", "manifests", "data-injection.yaml"))
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_0_Publish() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")

	// Publish skeleton package.
	noWaitExample := filepath.Join("examples", "helm-local-chart")
	ref := suite.Reference.String()
	_, stdErr, err := e2e.execZarfCommand("package", "publish", noWaitExample, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)
}

func (suite *SkeletonSuite) Test_1_Compose() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, _, err := e2e.execZarfCommand("package", "create", importEverything, "--confirm", "-o", "build", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_2_Deploy() {
	suite.T().Log("E2E: Created Skeleton Package Deploy")

	p := filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.arch))
	deploy := func(component string) error {
		_, _, err := e2e.execZarfCommand("package", "deploy", p, fmt.Sprintf("--components=%s", component), "--confirm")
		return err
	}
	remove := func(component string) error {
		_, _, err := e2e.execZarfCommand("package", "remove", "import-everything", fmt.Sprintf("--components=%s", component), "--confirm")
		return err
	}

	err := deploy("import-component-local")
	suite.NoError(err)

	err = remove("import-component-local")
	suite.NoError(err)

	err = deploy("import-component-local-relative")
	suite.NoError(err)

	err = remove("import-component-local-relative")
	suite.NoError(err)

	err = deploy("import-component-local-absolute")
	suite.NoError(err)

	err = deploy("import-component-oci")
	suite.NoError(err)

	// Verify that nginx successfully deploys in the cluster
	kubectlOut, _, _ := e2e.execZarfCommand("tools", "kubectl", "-n=local-chart", "rollout", "status", "deployment/local-demo")
	suite.Contains(string(kubectlOut), "successfully rolled out")

	err = deploy("import-helm")
	suite.NoError(err)

	err = remove("import-helm")
	suite.NoError(err)

	err = deploy("import-repos")
	suite.NoError(err)

	err = remove("import-repos")
	suite.NoError(err)

	err = deploy("import-images")
	suite.NoError(err)

	err = remove("import-images")
	suite.NoError(err)

	err = deploy("import-data-injections")
	suite.NoError(err)

	err = remove("import-data-injections")
	suite.NoError(err)
}

// func (suite *SkeletonSuite) Test_3_BadImports() {

func TestSkeletonSuite(t *testing.T) {
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)
	suite.Run(t, new(SkeletonSuite))
}

// to test:
// publish skeleton package
// compose skeleton package
// deploy newly created package
// publish newly created package
