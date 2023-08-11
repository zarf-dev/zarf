// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
	"github.com/stretchr/testify/require"

	// _ "github.com/distribution/distribution/v3/registry/auth/htpasswd"
	_ "github.com/distribution/distribution/v3/registry/storage/driver/inmemory"
)

// type testFileContents map[string]interface{}

// var testFiles = testFileContents{
// 	zconfig.ZarfYAML: types.ZarfPackage{
// 		Kind: types.ZarfPackageConfig,
// 		Metadata: types.ZarfMetadata{
// 			Name:    "oci-lib-unit-test",
// 			Version: "0.0.1",
// 		},
// 		Build: types.ZarfBuildData{
// 			Architecture: runtime.GOARCH,
// 		},
// 		Components: []types.ZarfComponent{
// 			{
// 				Name:     "nginx-remote",
// 				Required: true,
// 				Manifests: []types.ZarfManifest{
// 					{
// 						Name:      "simple-nginx-deployment",
// 						Namespace: "nginx",
// 						Files:     []string{"https://k8s.io/examples/application/deployment.yaml@c57f73449b26eae02ca2a549c388807d49ef6d3f2dc040a9bbb1290128d97157"},
// 					},
// 				},
// 				Images: []string{
// 					"nginx:1.14.2",
// 				},
// 			},
// 		},
// 	},
// 	zconfig.ZarfYAMLSignature: "signature contents",
// 	zconfig.ZarfSBOMTar:       "sboms.tar contents",
// 	filepath.Join(zconfig.ZarfComponentsDir, "nginx-remote.tar"): "nginx-remote.tar contents",
// 	ZarfPackageLayoutPath: ocispec.ImageLayout{
// 		Version: ocispec.ImageLayoutVersion,
// 	},
// 	ZarfPackageIndexPath: ocispec.Index{
// 		Manifests: []ocispec.Descriptor{
// 			{
// 				MediaType: ocispec.MediaTypeImageConfig,
// 				Digest: digest.FromString(),
// 			},
// 		},
// 	},
// }

// func setupDummyPackage(t *testing.T) string {

// 	tmp := t.TempDir()

// 	for name, contents := range testFiles {
// 		path := filepath.Join(tmp, name)
// 		var b []byte
// 		var err error
// 		switch contents.(type) {
// 		case string:
// 			b = []byte(contents.(string))
// 		case types.ZarfPackage:
// 			b, err = goyaml.Marshal(contents)
// 			require.NoError(t, err)
// 		default:
// 			b, err = json.Marshal(contents)
// 			require.NoError(t, err)
// 		}

// 		err = utils.WriteFile(path, b)
// 		require.NoError(t, err)
// 	}

// 	return tmp
// }

func setup(t *testing.T, port int) (*OrasRemote, *registry.Registry, context.CancelFunc) {
	ctx := context.TODO()
	ctx, cancel := context.WithCancel(ctx)

	config := &configuration.Configuration{}
	config.HTTP.Addr = fmt.Sprintf(":%d", port)
	config.HTTP.DrainTimeout = time.Duration(10) * time.Second
	config.Storage = map[string]configuration.Parameters{"inmemory": map[string]interface{}{}}

	reg, err := registry.NewRegistry(ctx, config)
	require.NoError(t, err)

	repo := "test"
	tag := "0.0.1"
	url := fmt.Sprintf("oci://localhost:%d/%s:%s", port, repo, tag)
	remote, err := NewOrasRemote(url)
	require.NoError(t, err)
	remote.ctx = ctx

	// populate the registry with a test package
	// dir := setupDummyPackage(t)
	// err = remote.PublishPackage(testFiles[zconfig.ZarfYAML].(*types.ZarfPackage), dir, 3)
	// require.NoError(t, err)

	return remote, reg, cancel
}

func Test_NewOrasRemote(t *testing.T) {
	// this is purposefully a basic test, as this functionality is
	// extensively tested in registry.ParseReference

	// should error with non-existent repository
	_, err := NewOrasRemote("oci://localhost:555")
	require.Error(t, err)

	// should error with a bad reference
	_, err = NewOrasRemote("oci://localhost:555/foo:bar/baz")
	require.Error(t, err)

	// should not error with a valid reference that does not exist
	remote, err := NewOrasRemote("oci://localhost:555/foo")
	require.NoError(t, err)

	todo := context.TODO()
	withCancel, cancel := context.WithCancel(todo)
	defer cancel()
	remote.WithContext(withCancel)
	require.Equal(t, withCancel, remote.ctx)
}
