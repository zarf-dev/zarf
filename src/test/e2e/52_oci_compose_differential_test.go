// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

// OCIDifferentialSuite validates that OCI imported components get handled correctly when performing a `zarf package create --differential`
type OCIDifferentialSuite struct {
	suite.Suite
	require.Assertions
	Remote    *utils.OrasRemote
	Reference registry.Reference
	tmpdir    string
}

var (
	differentialPackageName = ""
	normalPackageName       = ""
	examplePackagePath      = filepath.Join("examples", "helm-charts")
	anotherPackagePath      = filepath.Join("src", "test", "packages", "52-oci-differential")
)

func (suite *OCIDifferentialSuite) SetupSuite() {
	suite.tmpdir = suite.T().TempDir()
	suite.Reference.Registry = "localhost:555"
	differentialPackageName = fmt.Sprintf("zarf-package-podinfo-with-oci-flux-%s-v0.24.0-differential-v0.25.0.tar.zst", e2e.Arch)
	normalPackageName = fmt.Sprintf("zarf-package-podinfo-with-oci-flux-%s-v0.24.0.tar.zst", e2e.Arch)

	_ = e2e.SetupDockerRegistry(suite.T(), 555)

	// publish one of the example packages to the registry
	stdOut, stdErr, err := e2e.Zarf("package", "publish", examplePackagePath, "oci://"+suite.Reference.String(), "--insecure")
	suite.NoError(err, stdOut, stdErr)

	// build the package that we are going to publish
	stdOut, stdErr, err = e2e.Zarf("package", "create", anotherPackagePath, "--insecure", "--set=PACKAGE_VERSION=v0.24.0", "-o", suite.tmpdir, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// publish the package that we just built
	normalPackagePath := filepath.Join(suite.tmpdir, normalPackageName)
	stdOut, stdErr, err = e2e.Zarf("package", "publish", normalPackagePath, "oci://"+suite.Reference.String(), "--insecure")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *OCIDifferentialSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
}

func (suite *OCIDifferentialSuite) Test_0_Create_Differential_OCI() {
	suite.T().Log("E2E: Test Differential Packages w/ OCI Imports")

	// Build without differential
	stdOut, stdErr, err := e2e.Zarf("package", "create", anotherPackagePath, "--insecure", "--set=PACKAGE_VERSION=v0.25.0", "-o", suite.tmpdir, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Extract and load the zarf.yaml config for the normally built package
	err = archiver.Extract(filepath.Join(suite.tmpdir, normalPackageName), "zarf.yaml", suite.tmpdir)
	suite.NoError(err, "unable to extract zarf.yaml from the differential git package")
	var normalZarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(suite.tmpdir, "zarf.yaml"), &normalZarfConfig)
	suite.NoError(err, "unable to read zarf.yaml from the differential git package")
	os.Remove(filepath.Join(suite.tmpdir, "zarf.yaml"))

	stdOut, stdErr, err = e2e.Zarf("package", "create", anotherPackagePath, "--differential", "oci://"+suite.Reference.String()+"/podinfo-with-oci-flux:v0.24.0-amd64", "--insecure", "--set=PACKAGE_VERSION=v0.25.0", "-o", suite.tmpdir, "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Extract and load the zarf.yaml config for the differentially built package
	err = archiver.Extract(filepath.Join(suite.tmpdir, differentialPackageName), "zarf.yaml", suite.tmpdir)
	suite.NoError(err, "unable to extract zarf.yaml from the differential git package")
	var differentialZarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(suite.tmpdir, "zarf.yaml"), &differentialZarfConfig)
	suite.NoError(err, "unable to read zarf.yaml from the differential git package")

	/* Perform a bunch of asserts around the non-differential package */
	// Check the metadata and build data for the normal package
	suite.Equal(normalZarfConfig.Metadata.Version, "v0.24.0")
	suite.False(normalZarfConfig.Build.Differential)
	suite.Len(normalZarfConfig.Build.OCIImportedComponents, 1)
	suite.Equal(normalZarfConfig.Build.OCIImportedComponents["oci://127.0.0.1:555/helm-charts:0.0.1-skeleton"], "demo-helm-oci-chart")

	// Check the component data for the normal package
	suite.Len(normalZarfConfig.Components, 3)
	suite.Equal(normalZarfConfig.Components[0].Name, "demo-helm-oci-chart")
	suite.Equal(normalZarfConfig.Components[0].Charts[0].URL, "oci://ghcr.io/stefanprodan/charts/podinfo")
	suite.Equal(normalZarfConfig.Components[0].Images[0], "ghcr.io/stefanprodan/podinfo:6.3.3")
	suite.Len(normalZarfConfig.Components[1].Images, 2)
	suite.Len(normalZarfConfig.Components[1].Repos, 4)
	suite.Len(normalZarfConfig.Components[2].Images, 1)
	suite.Len(normalZarfConfig.Components[2].Repos, 3)

	/* Perform a bunch of asserts around the differential package */
	// Check the metadata and build data for the differential package
	suite.Equal(differentialZarfConfig.Metadata.Version, "v0.25.0")
	suite.True(differentialZarfConfig.Build.Differential)
	suite.Len(differentialZarfConfig.Build.DifferentialMissing, 1)
	suite.Equal(differentialZarfConfig.Build.DifferentialMissing[0], "demo-helm-oci-chart")
	suite.Len(differentialZarfConfig.Build.OCIImportedComponents, 0)

	// Check the component data for the differential package
	suite.Len(differentialZarfConfig.Components, 2)
	suite.Equal(differentialZarfConfig.Components[0].Name, "versioned-assets")
	suite.Len(differentialZarfConfig.Components[0].Images, 1)
	suite.Equal(differentialZarfConfig.Components[0].Images[0], "ghcr.io/defenseunicorns/zarf/agent:v0.25.0")
	suite.Len(differentialZarfConfig.Components[0].Repos, 1)
	suite.Equal(differentialZarfConfig.Components[0].Repos[0], "https://github.com/defenseunicorns/zarf.git@refs/tags/v0.25.0")
	suite.Len(differentialZarfConfig.Components[1].Images, 1)
	suite.Len(differentialZarfConfig.Components[1].Repos, 3)
	suite.Equal(differentialZarfConfig.Components[1].Images[0], "ghcr.io/stefanprodan/podinfo:latest")
	suite.Equal(differentialZarfConfig.Components[1].Repos[0], "https://github.com/stefanprodan/podinfo.git")

}

func TestOCIDifferentialSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(OCIDifferentialSuite))
}
