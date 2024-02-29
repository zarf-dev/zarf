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

func (suite *OCISuite) setupInMemoryRegistry(ctx context.Context) {
	port, err := freeport.GetFreePort()
	suite.NoError(err)
	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.Log.AccessLog.Disabled = true
	config.Log.Level = "error"
	config.HTTP.DrainTimeout = 10 * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	ref, err := registry.NewRegistry(ctx, config)
	suite.NoError(err)

	go ref.ListenAndServe()

	suite.registryURL = fmt.Sprintf("oci://localhost:%d/package:1.0.1", port)
}

func (suite *OCISuite) SetupTest() {
	// Registry config
	ctx := context.TODO()

	suite.setupInMemoryRegistry(ctx)

	platform := PlatformForArch("fake-package-so-does-not-matter")
	var err error
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

func (suite *OCISuite) TestCopyToTarget() {
	suite.T().Log("")
	ctx := context.TODO()

	// So what are the options.
	// completely tear down the registry and bring it back up before we do anything
	// have a long running ordered registry and do things as I go. For example in the index case

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

	dst, err := file.New(otherTempDir)
	suite.NoError(err)

	suite.NoError(err)

	// Testing copy to target
	suite.NoError(err)
	err = suite.remote.CopyToTarget(ctx, descs, dst, suite.remote.GetDefaultCopyOpts())
	suite.NoError(err)

	ociFile := filepath.Join(otherTempDir, ociFileName)
	b, err := os.ReadFile(ociFile)
	suite.NoError(err)
	contents := string(b)
	suite.Equal(contents, fileContents)
}

func (suite *OCISuite) TestPulledPaths() {
	suite.T().Log("")
	ctx := context.TODO()
	srcTempDir := suite.T().TempDir()
	files := []string{"firstFile", "secondFile"}

	var descs []ocispec.Descriptor
	src, err := file.New(srcTempDir)
	suite.NoError(err)
	for _, file := range files {
		path := filepath.Join(srcTempDir, file)
		os.Create(path)
		desc, err := src.Add(ctx, file, ocispec.MediaTypeEmptyJSON, path)
		suite.NoError(err)
		descs = append(descs, desc)
	}

	suite.publishPackage(src, descs)
	dstTempDir := suite.T().TempDir()

	// Testing pulled paths
	suite.remote.PullPaths(ctx, dstTempDir, files)
	suite.NoError(err)
	for _, file := range files {
		pulledPathOCIFile := filepath.Join(dstTempDir, file)
		_, err := os.Stat(pulledPathOCIFile)
		suite.NoError(err)
	}

}

func (suite *OCISuite) TestResolveRoot() {
	suite.T().Log("")
	ctx := context.TODO()
	srcTempDir := suite.T().TempDir()
	files := []string{"firstFile", "secondFile", "thirdFile"}

	var descs []ocispec.Descriptor
	src, err := file.New(srcTempDir)
	suite.NoError(err)
	for _, file := range files {
		path := filepath.Join(srcTempDir, file)
		os.Create(path)
		desc, err := src.Add(ctx, file, ocispec.MediaTypeEmptyJSON, path)
		suite.NoError(err)
		descs = append(descs, desc)
	}

	suite.publishPackage(src, descs)

	root, err := suite.remote.FetchRoot(ctx)
	suite.NoError(err)
	b, err := root.MarshalJSON()
	suite.NoError(err)
	suite.Equal(3, len(root.Layers))
	fmt.Printf("this is the root %v", string(b))
	fmt.Print("done with root\n")
	desc := root.Locate("thirdFile")
	suite.Equal("thirdFile", desc.Annotations[ocispec.AnnotationTitle])
}

func (suite *OCISuite) TestCopy() {
	suite.T().Log("")
	ctx := context.TODO()
	srcTempDir := suite.T().TempDir()
	files := []string{"firstFile"}

	var descs []ocispec.Descriptor
	src, err := file.New(srcTempDir)
	suite.NoError(err)
	for _, file := range files {
		path := filepath.Join(srcTempDir, file)
		os.Create(path)
		desc, err := src.Add(ctx, file, ocispec.MediaTypeEmptyJSON, path)
		suite.NoError(err)
		descs = append(descs, desc)
	}

	suite.publishPackage(src, descs)

	otherSrc, err := file.New(srcTempDir)
	suite.NoError(err)
	for _, file := range files {
		path := filepath.Join(srcTempDir, file)
		os.Create(path)
		desc, err := otherSrc.Add(ctx, file, ocispec.MediaTypeEmptyJSON, path)
		suite.NoError(err)
		descs = append(descs, desc)
	}

	// Everything we want to test
	// Copy
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
