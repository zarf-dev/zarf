// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with artifacts stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

}

func (suite *OCISuite) SetupTest() {
	// Registry config
	ctx := context.TODO()
	config := &configuration.Configuration{}
	port, err := freeport.GetFreePort()
	suite.NoError(err)

	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	ref, err := registry.NewRegistry(ctx, config)
	suite.NoError(err)

	go ref.ListenAndServe()

	suite.registryURL = fmt.Sprintf("oci://localhost:%d/package:1.0.1", port)

	platform := PlatformForArch("fake-package-so-does-not-matter")
	suite.remote, err = NewOrasRemote(suite.registryURL, platform, WithPlainHTTP(true))
	suite.NoError(err)
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

func (suite *OCISuite) publishPackage(src *file.Store, descs []ocispec.Descriptor) {
	ctx := context.TODO()
	annotations := map[string]string{
		ocispec.AnnotationTitle:       "name",
		ocispec.AnnotationDescription: "description",
	}

	manifestConfigDesc, err := suite.remote.CreateAndPushManifestConfig(ctx, annotations, ocispec.MediaTypeLayoutHeader)
	suite.NoError(err)

	manifestDesc, err := suite.remote.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	suite.NoError(err)
	publishedDesc, err := oras.Copy(ctx, src, manifestDesc.Digest.String(), suite.remote.Repo(), "", suite.remote.GetDefaultCopyOpts())
	suite.NoError(err)

	err = suite.remote.UpdateIndex(ctx, "0.0.1", publishedDesc)
	suite.NoError(err)
}

func (suite *OCISuite) TestPublishForReal() {
	suite.T().Log("")
	ctx := context.TODO()

	// So what are my options.
	// I could completely tear down the registry and bring it back up before we do anything
	// I could have a long running ordered registry and do things as I go. For example in the index case
	// I don't think I actually need a file to be in the source so the same title shouldn't actually matter

	// arrange

	srcTempDir := suite.T().TempDir()
	regularFileName := "this-file-is-in-a-regular-directory"
	fileContents := "here's what I'm putting in the file"
	ociFileName := "this-file-is-in-a-oci-file-store"

	regularFilePath := filepath.Join(srcTempDir, regularFileName)
	os.WriteFile(regularFilePath, []byte(fileContents), 0644)
	src, err := file.New(srcTempDir)
	suite.NoError(err)

	desc, err := src.Add(ctx, ociFileName, ocispec.MediaTypeEmptyJSON, regularFilePath)
	suite.NoError(err)

	descs := []ocispec.Descriptor{desc}
	suite.publishPackage(src, descs)

	otherTempDir := suite.T().TempDir()
	thirdTempDir := suite.T().TempDir()

	dst, err := file.New(otherTempDir)
	suite.NoError(err)

	suite.NoError(err)

	// Act

	// otherPlatform := PlatformForArch("other")
	// suite.remote.targetPlatform = &otherPlatform
	// annotations[ocispec.AnnotationTitle] = "otherName"
	// otherManifestConfig, err := suite.remote.CreateAndPushManifestConfig(ctx, annotations, ocispec.MediaTypeLayoutHeader)
	// suite.NoError(err)

	// _, err = suite.remote.PackAndTagManifest(ctx, src, descs, otherManifestConfig, annotations)
	// suite.NoError(err)

	// publishedOtherManifest, err := oras.Copy(ctx, src, manifestDesc.Digest.String(), suite.remote.Repo(), "", suite.remote.GetDefaultCopyOpts())
	// suite.NoError(err)

	// err = suite.remote.UpdateIndex(ctx, "0.0.1", publishedOtherManifest)
	// suite.NoError(err)

	// Testing copy to target
	suite.NoError(err)
	err = suite.remote.CopyToTarget(ctx, descs, dst, suite.remote.GetDefaultCopyOpts())
	suite.NoError(err)

	// Assert
	ociFile := filepath.Join(otherTempDir, ociFileName)
	b, err := os.ReadFile(ociFile)
	suite.NoError(err)
	contents := string(b)
	suite.Equal(contents, fileContents)

	// Testing pulled paths
	pulledDescs, err := suite.remote.PullPaths(ctx, thirdTempDir, []string{ociFileName, "path-that-does-not-exist"})
	fmt.Printf("pulled descs %v", pulledDescs)
	suite.NoError(err)
	pulledPathOCIFile := filepath.Join(thirdTempDir, ociFileName)
	b, err = os.ReadFile(pulledPathOCIFile)
	suite.NoError(err)
	contents = string(b)
	suite.Equal(contents, fileContents)

	// Testing fetch root
	// suite.remote.root = nil
	// suite.Nil(suite.remote.root)
	root, err := suite.remote.FetchRoot(ctx)
	suite.NoError(err)
	fmt.Printf("this is the root %v", root)
	// suite.Equal(root.Config.Digest, otherManifestConfig.Digest)

	// Testing resolve root
	rootDesc, err := suite.remote.ResolveRoot(ctx)
	suite.NoError(err)
	suite.Equal(ocispec.MediaTypeImageManifest, rootDesc.MediaType)

	// Everything we want to test
	// Copy
	// Manifest locate
	// Index more in depth
}

func TestRemoveDuplicateDescriptors(t *testing.T) {
	tests := []struct {
		name     string
		input    []ocispec.Descriptor
		expected []ocispec.Descriptor
	}{
		{
			name: "no duplicates",
			input: []ocispec.Descriptor{
				{Digest: "sha256:1111", Size: 100},
				{Digest: "sha256:2222", Size: 200},
			},
			expected: []ocispec.Descriptor{
				{Digest: "sha256:1111", Size: 100},
				{Digest: "sha256:2222", Size: 200},
			},
		},
		{
			name: "with duplicates",
			input: []ocispec.Descriptor{
				{Digest: "sha256:1111", Size: 100},
				{Digest: "sha256:1111", Size: 100},
				{Digest: "sha256:2222", Size: 200},
			},
			expected: []ocispec.Descriptor{
				{Digest: "sha256:1111", Size: 100},
				{Digest: "sha256:2222", Size: 200},
			},
		},
		{
			name: "with empty descriptor",
			input: []ocispec.Descriptor{
				{Size: 0},
				{Digest: "sha256:1111", Size: 100},
			},
			expected: []ocispec.Descriptor{
				{Digest: "sha256:1111", Size: 100},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveDuplicateDescriptors(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("RemoveDuplicateDescriptors(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestOCI(t *testing.T) {
	suite.Run(t, new(OCISuite))
}
