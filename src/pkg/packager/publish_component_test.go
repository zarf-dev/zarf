// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v3/cmd/cosign/cli/verify"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"oras.land/oras-go/v2/registry/remote"
)

func TestPublishComponent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	published, err := PublishComponent(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component.yaml"), createRegistry(ctx, t), PublishComponentOptions{
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	repo, err := remote.NewRepository(published.Registry + "/" + published.Repository)
	require.NoError(t, err)
	repo.PlainHTTP = true
	descriptor, err := repo.Resolve(ctx, published.Reference)
	require.NoError(t, err)
	require.Equal(t, ocispec.MediaTypeImageManifest, descriptor.MediaType)
	require.NotZero(t, descriptor.Size)

	manifestReader, err := repo.Manifests().Fetch(ctx, descriptor)
	require.NoError(t, err)
	defer func() { require.NoError(t, manifestReader.Close()) }()
	manifestBytes, err := io.ReadAll(manifestReader)
	require.NoError(t, err)
	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.Equal(t, ComponentConfigMediaType, manifest.Config.MediaType)
	require.NotEmpty(t, manifest.Layers)

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations[ocispec.AnnotationTitle])
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

func TestPublishComponentFlavor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	published, err := PublishComponent(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component-flavor.yaml"), createRegistry(ctx, t), PublishComponentOptions{
		Flavor:        "test",
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	require.Equal(t, "0.0.1-test", published.Reference)

	repo, err := remote.NewRepository(published.Registry + "/" + published.Repository)
	require.NoError(t, err)
	repo.PlainHTTP = true
	descriptor, err := repo.Resolve(ctx, published.Reference)
	require.NoError(t, err)
	manifestReader, err := repo.Manifests().Fetch(ctx, descriptor)
	require.NoError(t, err)
	defer func() { require.NoError(t, manifestReader.Close()) }()
	manifestBytes, err := io.ReadAll(manifestReader)
	require.NoError(t, err)
	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	componentReader, err := repo.Blobs().Fetch(ctx, manifest.Config)
	require.NoError(t, err)
	defer func() { require.NoError(t, componentReader.Close()) }()
	componentBytes, err := io.ReadAll(componentReader)
	require.NoError(t, err)
	var component v1beta1.ComponentConfig
	require.NoError(t, goyaml.Unmarshal(componentBytes, &component))
	require.Empty(t, component.Component.Selector.Flavor)
}

func TestPublishComponentSigning(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	signOpts := signing.DefaultSignBlobOptions()
	signOpts.Key = filepath.Join("testdata", "publish", "cosign.key")
	signOpts.Password = "password"
	signOpts.SkipConfirmation = true
	published, err := PublishComponent(ctx, filepath.Join("testdata", "publish-component-v1beta1", "component-flavor.yaml"), createRegistry(ctx, t), PublishComponentOptions{
		Flavor:          "test",
		SignBlobOptions: signOpts,
		RemoteOptions:   defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	require.NoError(t, verifyPublishedComponentSignature(ctx, published.String(), filepath.Join("testdata", "publish", "cosign.pub")))
}

func TestPublishComponentRejectsNegativeRetries(t *testing.T) {
	t.Parallel()

	_, err := PublishComponent(context.Background(), filepath.Join("testdata", "publish-component-v1beta1", "component.yaml"), createRegistry(context.Background(), t), PublishComponentOptions{
		Retries:       -1,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.EqualError(t, err, "retries cannot be negative")
}

func TestPublishComponentImageArchivesUseOCILayout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	archivePath := filepath.Join(t.TempDir(), "images.tar")
	imageLayout := filepath.Join("..", "images", "testdata", "oras-oci-layout", "images")
	require.NoError(t, archive.Compress(ctx, []string{imageLayout}, archivePath, archive.CompressOpts{}))
	componentPath := filepath.Join(t.TempDir(), "component.yaml")
	// FIXME: make this readable
	componentYAML := []byte("apiVersion: zarf.dev/v1beta1\nkind: ZarfComponentConfig\nmetadata:\n  name: image-archive-component\n  version: 0.0.1\ncomponent:\n  imageArchives:\n    - path: " + archivePath + "\n      images:\n        - ghcr.io/zarf-dev/images/hello-world:latest\n")
	require.NoError(t, os.WriteFile(componentPath, componentYAML, 0o600))

	published, err := PublishComponent(ctx, componentPath, createRegistry(ctx, t), PublishComponentOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)

	repo, err := remote.NewRepository(published.Registry + "/" + published.Repository)
	require.NoError(t, err)
	repo.PlainHTTP = true
	descriptor, err := repo.Resolve(ctx, published.Reference)
	require.NoError(t, err)
	manifestReader, err := repo.Manifests().Fetch(ctx, descriptor)
	require.NoError(t, err)
	defer func() { require.NoError(t, manifestReader.Close()) }()
	manifestBytes, err := io.ReadAll(manifestReader)
	require.NoError(t, err)
	var manifest ocispec.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	layerTitles := make([]string, 0, len(manifest.Layers))
	for _, layer := range manifest.Layers {
		layerTitles = append(layerTitles, layer.Annotations[ocispec.AnnotationTitle])
	}
	require.Contains(t, layerTitles, "images/oci-layout")
	require.Contains(t, layerTitles, "images/index.json")
	require.NotContains(t, layerTitles, "images.tar")
	require.Contains(t, layerTitles, "images/blobs/sha256/03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c")
}

func verifyPublishedComponentSignature(ctx context.Context, componentRef, publicKeyPath string) error {
	cmd := &verify.VerifyCommand{
		RegistryOptions: options.RegistryOptions{AllowHTTPRegistry: true},
		CommonVerifyOptions: options.CommonVerifyOptions{
			IgnoreTlog:      true,
			NewBundleFormat: true,
		},
		CheckClaims:     true,
		KeyRef:          publicKeyPath,
		IgnoreTlog:      true,
		NewBundleFormat: true,
	}
	return cmd.Exec(ctx, []string{componentRef})
}

func TestComponentResourcesRejectUnsupportedRemoteSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component v1beta1.ComponentConfig
		wantErr   string
	}{
		{
			name: "values file",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Files: []string{"https://example.com/values.yaml"}},
			},
			wantErr: "remote values files are not supported",
		},
		{
			name: "values schema",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Schema: "https://example.com/values.schema.json"},
			},
			wantErr: "remote values schemas are not supported",
		},
		{
			name: "local chart",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Charts: []v1beta1.Chart{{Local: &v1beta1.LocalSource{Path: "https://example.com/chart.tgz"}}}},
			},
			wantErr: "remote local chart paths are not supported",
		},
		{
			name: "image archive",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{{Path: "https://example.com/images.tar"}}},
			},
			wantErr: "remote image archive paths are not supported",
		},
		{
			name: "component import",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Import: v1beta1.ComponentImport{Remote: []v1beta1.ComponentImportRemote{{URL: "oci://example.com/components/foo:1.0.0"}}}},
			},
			wantErr: "remote component imports are not yet supported for v1beta1 packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := componentResources(filepath.Join(t.TempDir(), "component.yaml"), tt.component, t.TempDir(), map[string]bool{})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestComponentResourcesAllowsSupportedRemoteSources(t *testing.T) {
	t.Parallel()

	component := v1beta1.ComponentConfig{
		Component: v1beta1.ComponentSpec{
			Charts: []v1beta1.Chart{{ValuesFiles: []v1beta1.ValuesFile{{Path: "https://example.com/chart-values.yaml"}}}},
			Manifests: []v1beta1.Manifest{{
				Files:     []string{"https://example.com/manifest.yaml"},
				Kustomize: &v1beta1.KustomizeManifest{Files: []string{"https://example.com/kustomization.yaml"}},
			}},
			Files: []v1beta1.File{{Source: "https://example.com/file.txt"}},
		},
	}
	resources, err := componentResources(filepath.Join(t.TempDir(), "component.yaml"), component, t.TempDir(), map[string]bool{})
	require.NoError(t, err)
	require.Empty(t, resources)
}
