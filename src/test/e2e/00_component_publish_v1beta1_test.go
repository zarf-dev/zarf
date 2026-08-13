// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry/remote"
)

func TestPublishV1Beta1Component(t *testing.T) {
	t.Parallel()

	registryAddress := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	componentPath := filepath.Join("src", "test", "packages", "00-component-publish-v1beta1", "component.yaml")

	stdout, stderr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryAddress+"/components", "--plain-http", "--no-color")
	require.NoError(t, err, stdout, stderr)

	repo, err := remote.NewRepository(registryAddress + "/components/published-component")
	require.NoError(t, err)
	repo.PlainHTTP = true
	descriptor, err := repo.Resolve(testutil.TestContext(t), "0.0.1")
	require.NoError(t, err)
	require.Equal(t, "application/vnd.oci.image.manifest.v1+json", descriptor.MediaType)
	require.NotZero(t, descriptor.Size)

	manifestReader, err := repo.Manifests().Fetch(testutil.TestContext(t), descriptor)
	require.NoError(t, err)
	defer func() { require.NoError(t, manifestReader.Close()) }()
	manifestBytes, err := io.ReadAll(manifestReader)
	require.NoError(t, err)
	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.Equal(t, packager.ComponentConfigMediaType, manifest.Config.MediaType)
	require.NotEmpty(t, manifest.Layers)

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations["org.opencontainers.image.title"])
	}
	for _, localPath := range []string{
		"component-values.yaml",
		"component-values.schema.json",
		"local-chart-values.yaml",
		"local-chart/Chart.yaml",
		"local-chart/templates/configmap.yaml",
		"local-file.txt",
		"local-manifest.yaml",
	} {
		require.Contains(t, layerTitles, localPath)
	}
	for _, remotePath := range []string{
		"https://example.com/remote-chart-values.yaml",
		"https://example.com/remote-file.txt",
		"https://example.com/remote-manifest.yaml",
	} {
		require.NotContains(t, layerTitles, remotePath)
	}
}
