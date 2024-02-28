// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with artifacts stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory" // used for docker test registry
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

type OCISuite struct {
	suite.Suite
	*require.Assertions
	remote      *OrasRemote
	registryURL string
}

func (suite *OCISuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	suite.StartRegistry()

	platform := PlatformForArch("fake-package-so-does-not-matter")
	var err error
	suite.remote, err = NewOrasRemote(suite.registryURL, platform, WithPlainHTTP(true))
	suite.NoError(err)

}

func (suite *OCISuite) StartRegistry() {
	// Registry config
	ctx := context.TODO()
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.NoError(err)

	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	ref, err := registry.NewRegistry(ctx, config)
	suite.NoError(err)

	go ref.ListenAndServe()

	suite.registryURL = fmt.Sprintf("oci://localhost:%d/package:1.0.1", port)
}

func (suite *OCISuite) TestBadRemote() {
	suite.T().Log("Here")
	_, err := NewOrasRemote("nonsense", PlatformForArch("fake-package-so-does-not-matter"))
	suite.Error(err)
}

func (suite *OCISuite) TestPublishFailNoTitle() {
	suite.T().Log("")

	ctx := context.TODO()
	annotations := map[string]string{
		ocispec.AnnotationDescription: "No title",
	}
	_, err := suite.remote.CreateAndPushManifestConfig(ctx, annotations, ocispec.MediaTypeImageConfig)
	suite.Error(err)
}

func (suite *OCISuite) TestPublishSuccess() {
	suite.T().Log("")

	ctx := context.TODO()
	annotations := map[string]string{
		ocispec.AnnotationTitle:       "name",
		ocispec.AnnotationDescription: "description",
	}

	_, err := suite.remote.CreateAndPushManifestConfig(ctx, annotations, ocispec.MediaTypeImageConfig)
	suite.NoError(err)

}

func (suite *OCISuite) TestPublishForReal() {
	suite.T().Log("")

	// arrange
	ctx := context.TODO()
	annotations := map[string]string{
		ocispec.AnnotationTitle:       "name",
		ocispec.AnnotationDescription: "description",
	}

	srcTempDir := suite.T().TempDir()

	fileContents := "here's what I'm putting in the file"
	regularFileName := "this-file-is-in-a-regular-directory"
	ociFileName := "this-file-is-in-a-oci-file-store"

	regularFilePath := filepath.Join(srcTempDir, regularFileName)
	os.WriteFile(regularFilePath, []byte(fileContents), 0644)
	src, err := file.New(srcTempDir)
	suite.NoError(err)
	desc, err := src.Add(ctx, ociFileName, ocispec.MediaTypeEmptyJSON, regularFilePath)
	suite.NoError(err)
	descs := []ocispec.Descriptor{desc}

	// Act
	manifestConfigDesc, err := suite.remote.CreateAndPushManifestConfig(ctx, annotations, ocispec.MediaTypeLayoutHeader)
	suite.NoError(err)

	manifestDesc, err := suite.remote.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	suite.NoError(err)
	publishedDesc, err := oras.Copy(ctx, src, manifestDesc.Digest.String(), suite.remote.Repo(), "", suite.remote.GetDefaultCopyOpts())
	suite.NoError(err)

	err = suite.remote.UpdateIndex(ctx, "0.0.1", publishedDesc)
	suite.NoError(err)

	otherTempDir := suite.T().TempDir()
	dst, err := file.New(otherTempDir)

	suite.NoError(err)
	err = suite.remote.CopyToTarget(ctx, descs, dst, suite.remote.GetDefaultCopyOpts())
	suite.NoError(err)

	// Assert
	ociFile := filepath.Join(otherTempDir, ociFileName)
	b, err := os.ReadFile(ociFile)
	suite.NoError(err)
	contents := string(b)
	suite.Equal(contents, fileContents)
}

func TestOCI(t *testing.T) {
	suite.Run(t, new(OCISuite))
}
