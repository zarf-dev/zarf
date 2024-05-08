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
	"time"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

type PublishDeploySuiteTestSuite struct {
	suite.Suite
	*require.Assertions
	Reference   registry.Reference
	PackagesDir string
}

var badDeployRef = registry.Reference{
	Registry:   "localhost:5000",
	Repository: "zarf-test",
	Reference:  "bad-tag",
}

func (suite *PublishDeploySuiteTestSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.PackagesDir = "build"

	e2e.SetupDockerRegistry(suite.T(), 555)
	suite.Reference.Registry = "localhost:555"
}

func (suite *PublishDeploySuiteTestSuite) TearDownSuite() {
	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	e2e.CleanFiles(local)

	e2e.TeardownRegistry(suite.T(), 555)
}

func (suite *PublishDeploySuiteTestSuite) Test_0_Publish() {
	suite.T().Log("E2E: Package Publish oci://")

	// Publish package.
	example := filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch))
	ref := suite.Reference.String()
	stdOut, stdErr, err := e2e.Zarf("package", "publish", example, "oci://"+ref, "--insecure")
	suite.NoError(err, stdOut, stdErr)
	suite.Contains(stdErr, "Published "+ref)

	// Pull the package via OCI.
	stdOut, stdErr, err = e2e.Zarf("package", "pull", "oci://"+ref+"/helm-charts:0.0.1", "--insecure")
	suite.NoError(err, stdOut, stdErr)

	// Publish w/ package missing `metadata.version` field.
	example = filepath.Join(suite.PackagesDir, fmt.Sprintf("zarf-package-component-actions-%s.tar.zst", e2e.Arch))
	_, stdErr, err = e2e.Zarf("package", "publish", example, "oci://"+ref, "--insecure")
	suite.Error(err, stdErr)

	// Inline publish package.
	dir := filepath.Join("examples", "helm-charts")
	stdOut, stdErr, err = e2e.Zarf("package", "create", dir, "-o", "oci://"+ref, "--insecure", "--oci-concurrency=5", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Inline publish flavor.
	dir = filepath.Join("examples", "package-flavors")
	stdOut, stdErr, err = e2e.Zarf("package", "create", dir, "-o", "oci://"+ref, "--flavor", "oracle-cookie-crunch", "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Inspect published flavor.
	stdOut, stdErr, err = e2e.Zarf("package", "inspect", "oci://"+ref+"/package-flavors:1.0.0-oracle-cookie-crunch", "--insecure")
	suite.NoError(err, stdOut, stdErr)

	// Inspect the published package.
	stdOut, stdErr, err = e2e.Zarf("package", "inspect", "oci://"+ref+"/helm-charts:0.0.1", "--insecure")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *PublishDeploySuiteTestSuite) Test_1_Deploy() {
	suite.T().Log("E2E: Package Deploy oci://")

	// Build the fully qualified reference.
	suite.Reference.Repository = "helm-charts"
	suite.Reference.Reference = "0.0.1"
	ref := suite.Reference.String()

	// Deploy the package via OCI.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", "oci://"+ref, "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Remove the package via OCI.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "oci://"+ref, "--insecure", "--confirm")
	suite.NoError(err, stdOut, stdErr)

	// Test deploy w/ bad ref.
	_, stdErr, err = e2e.Zarf("package", "deploy", "oci://"+badDeployRef.String(), "--insecure", "--confirm")
	suite.Error(err, stdErr)
}

func (suite *PublishDeploySuiteTestSuite) Test_2_Pull_And_Deploy() {
	suite.T().Log("E2E: Package Pull oci:// && Package Deploy tarball")

	local := fmt.Sprintf("zarf-package-helm-charts-%s-0.0.1.tar.zst", e2e.Arch)
	defer e2e.CleanFiles(local)
	// Verify the package was pulled.
	suite.FileExists(local)

	// Deploy the local package.
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", local, "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *PublishDeploySuiteTestSuite) Test_3_Copy() {
	t := suite.T()
	ref := suite.Reference.String()
	dstRegistryPort := 556
	dstRef := strings.Replace(ref, fmt.Sprint(555), fmt.Sprint(dstRegistryPort), 1)

	e2e.SetupDockerRegistry(t, dstRegistryPort)
	defer e2e.TeardownRegistry(t, dstRegistryPort)

	src, err := zoci.NewRemote(ref, oci.PlatformForArch(e2e.Arch), oci.WithPlainHTTP(true))
	suite.NoError(err)

	dst, err := zoci.NewRemote(dstRef, oci.PlatformForArch(e2e.Arch), oci.WithPlainHTTP(true))
	suite.NoError(err)

	reg, err := remote.NewRegistry(strings.Split(dstRef, "/")[0])
	suite.NoError(err)
	reg.PlainHTTP = true
	attempt := 0
	ctx := context.TODO()
	for attempt <= 5 {
		err = reg.Ping(ctx)
		if err == nil {
			break
		}
		attempt++
		time.Sleep(2 * time.Second)
	}
	require.Less(t, attempt, 5, "failed to ping registry")

	err = zoci.CopyPackage(ctx, src, dst, 5)
	suite.NoError(err)

	srcRoot, err := src.FetchRoot(ctx)
	suite.NoError(err)

	for _, layer := range srcRoot.Layers {
		ok, err := dst.Repo().Exists(ctx, layer)
		suite.True(ok)
		suite.NoError(err)
	}
}

func TestPublishDeploySuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(PublishDeploySuiteTestSuite))
}
