// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
)

func TestCopyToTargetUsesSelectedManifest(t *testing.T) {
	t.Parallel()

	for _, useIndex := range []bool{false, true} {
		name := "manifest"
		if useIndex {
			name = "index"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := testutil.TestContext(t)
			ref := registry.Reference{
				Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
				Repository: "packages",
				Reference:  "example",
			}
			platform := PlatformForArch("amd64")
			remote, err := NewOrasRemote(ref.String(), platform, WithPlainHTTP(true))
			require.NoError(t, err)

			source := memory.New()
			configBytes := []byte("{}")
			config := content.NewDescriptorFromBytes("application/vnd.zarf.config.v1+json", configBytes)
			require.NoError(t, source.Push(ctx, config, bytes.NewReader(configBytes)))
			publish := func(contents string) {
				t.Helper()

				layer := content.NewDescriptorFromBytes("application/vnd.zarf.layer.v1.blob", []byte(contents))
				layer.Annotations = map[string]string{ocispec.AnnotationTitle: "zarf.yaml"}
				require.NoError(t, source.Push(ctx, layer, bytes.NewReader([]byte(contents))))
				manifest, err := oras.PackManifest(ctx, source, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
					Layers:           []ocispec.Descriptor{layer},
					ConfigDescriptor: &config,
				})
				require.NoError(t, err)
				require.NoError(t, source.Tag(ctx, manifest, manifest.Digest.String()))
				_, err = oras.Copy(ctx, source, manifest.Digest.String(), remote.Repo(), "", remote.GetDefaultCopyOpts())
				require.NoError(t, err)
				if useIndex {
					manifest, err = UpdateIndexWithDescriptor(ctx, remote.Repo(), ref.Reference, platform, manifest)
					require.NoError(t, err)
				}
				require.NoError(t, remote.Repo().Tag(ctx, manifest, ref.Reference))
			}

			publish("original package definition")
			root, err := remote.FetchRoot(ctx)
			require.NoError(t, err)
			layers := []ocispec.Descriptor{root.Locate("zarf.yaml")}

			publish("replacement package definition")
			destination := t.TempDir()
			target, err := file.New(destination)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, target.Close()) })
			require.NoError(t, remote.CopyToTarget(ctx, layers, target, remote.GetDefaultCopyOpts()))
			contents, err := os.ReadFile(filepath.Join(destination, "zarf.yaml"))
			require.NoError(t, err)
			require.Equal(t, "original package definition", string(contents))
		})
	}
}

func TestCopyToTargetCopiesOnlyRequestedLayers(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "packages",
		Reference:  "example",
	}
	cache := memory.New()
	remote, err := NewOrasRemote(ref.String(), ocispec.Platform{}, WithPlainHTTP(true),
		WithCache(cache), WithRetryPolicy(testRetryPolicy(1)))
	require.NoError(t, err)

	source := memory.New()
	config := content.NewDescriptorFromBytes("application/vnd.zarf.package.config.v1+json", []byte("config"))
	requested := content.NewDescriptorFromBytes("application/vnd.zarf.package.layer.v1+tar", []byte("requested"))
	excluded := content.NewDescriptorFromBytes("application/vnd.zarf.package.layer.v1+tar", []byte("excluded"))
	require.NoError(t, source.Push(ctx, config, bytes.NewReader([]byte("config"))))
	require.NoError(t, source.Push(ctx, requested, bytes.NewReader([]byte("requested"))))
	require.NoError(t, source.Push(ctx, excluded, bytes.NewReader([]byte("excluded"))))
	manifest, err := oras.PackManifest(ctx, source, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		Layers:           []ocispec.Descriptor{requested, excluded},
		ConfigDescriptor: &config,
	})
	require.NoError(t, err)
	require.NoError(t, source.Tag(ctx, manifest, ref.Reference))
	_, err = oras.Copy(ctx, source, ref.Reference, remote.Repo(), ref.Reference, remote.GetDefaultCopyOpts())
	require.NoError(t, err)

	target := memory.New()
	require.NoError(t, remote.CopyToTarget(ctx, []ocispec.Descriptor{requested}, target, remote.GetDefaultCopyOpts()))
	exists, err := target.Exists(ctx, requested)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = target.Exists(ctx, excluded)
	require.NoError(t, err)
	require.False(t, exists)
	cached, err := content.FetchAll(ctx, cache, requested)
	require.NoError(t, err)
	require.Equal(t, []byte("requested"), cached)
}
