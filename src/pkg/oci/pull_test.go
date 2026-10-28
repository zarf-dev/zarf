// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry"
)

func TestCopyToTargetCopiesOnlyRequestedLayers(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	ref := registry.Reference{
		Registry:   testutil.SetupInMemoryRegistryDynamic(ctx, t),
		Repository: "packages",
		Reference:  "example",
	}
	remote, err := NewOrasRemote(ref.String(), ocispec.Platform{}, WithPlainHTTP(true))
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
}
