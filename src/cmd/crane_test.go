// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/errdef"
)

func TestRegistryPruneSkipsConnectedDeploys(t *testing.T) {
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	options := []crane.Option{crane.Insecure}

	keptDigest := testutil.PushImage(ctx, t, address+"/library/kept", "latest")
	unusedDigest := testutil.PushImage(ctx, t, address+"/library/unused", "latest")

	packages := []state.DeployedPackage{
		{
			PackageConnectivity: state.PackageConnectivityConnected,
			Data: v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{
				{Name: "connected", Images: []string{"docker.io/library/connected:latest"}},
			}},
			DeployedComponents: []state.DeployedComponent{{Name: "connected"}},
		},
		{
			Data: v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{
				{Name: "airgap", Images: []string{"docker.io/library/kept:latest"}},
			}},
			DeployedComponents: []state.DeployedComponent{{Name: "airgap"}},
		},
	}

	require.NoError(t, doPruneImagesForPackages(ctx, options, &state.State{}, packages, address, true, false))
	kept, err := testutil.NewRepo(t, address+"/library/kept").Resolve(ctx, "latest")
	require.NoError(t, err)
	require.Equal(t, keptDigest, kept.Digest.String())
	_, err = testutil.NewRepo(t, address+"/library/unused").Resolve(ctx, unusedDigest)
	require.ErrorIs(t, err, errdef.ErrNotFound)
}

func TestRegistryCopyPlatform(t *testing.T) {
	ctx := context.Background()

	t.Run("copies selected platform from index with OCI attestation", func(t *testing.T) {
		sourceRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
		destinationRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
		sourceRepo := testutil.NewRepo(t, sourceRegistry+"/source")
		destinationRepo := testutil.NewRepo(t, destinationRegistry+"/destination")

		amd64 := testutil.PushSinglePlatformImage(ctx, t, sourceRepo, "amd64")
		amd64.Platform = &ocispec.Platform{OS: "linux", Architecture: "amd64"}
		arm64 := testutil.PushSinglePlatformImage(ctx, t, sourceRepo, "arm64")
		arm64.Platform = &ocispec.Platform{OS: "linux", Architecture: "arm64"}

		config := testutil.PushBlob(ctx, t, sourceRepo, ocispec.MediaTypeEmptyJSON, []byte("{}"))
		layer := testutil.PushBlob(ctx, t, sourceRepo, "application/vnd.in-toto+json", []byte(`{"_type":"https://in-toto.io/Statement/v0.1"}`))
		artifact := ocispec.Manifest{
			Versioned:    specs.Versioned{SchemaVersion: 2},
			MediaType:    ocispec.MediaTypeImageManifest,
			ArtifactType: "application/vnd.in-toto+json",
			Config:       config,
			Layers:       []ocispec.Descriptor{layer},
			Subject:      &amd64,
		}
		artifactBody, err := json.Marshal(artifact)
		require.NoError(t, err)
		artifactDesc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Digest:    digest.FromBytes(artifactBody),
			Size:      int64(len(artifactBody)),
		}
		require.NoError(t, sourceRepo.Push(ctx, artifactDesc, bytes.NewReader(artifactBody)))

		index := testutil.PushIndex(ctx, t, sourceRepo, []ocispec.Descriptor{amd64, arm64, artifactDesc})
		require.NoError(t, sourceRepo.Tag(ctx, index, "attested"))

		cmd := newRegistryCommand()
		cmd.SetArgs([]string{"--insecure", "--platform", "linux/amd64", "copy", sourceRegistry + "/source:attested", destinationRegistry + "/destination:selected"})
		require.NoError(t, cmd.ExecuteContext(ctx))

		selected, err := destinationRepo.Resolve(ctx, "selected")
		require.NoError(t, err)
		require.Equal(t, amd64.Digest, selected.Digest)
		require.Equal(t, amd64.MediaType, selected.MediaType)
	})

	t.Run("copies whole index by default", func(t *testing.T) {
		sourceRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
		destinationRegistry := testutil.SetupInMemoryRegistryDynamic(ctx, t)
		sourceRepo := testutil.NewRepo(t, sourceRegistry+"/source")
		destinationRepo := testutil.NewRepo(t, destinationRegistry+"/destination")

		amd64 := testutil.PushSinglePlatformImage(ctx, t, sourceRepo, "amd64")
		amd64.Platform = &ocispec.Platform{OS: "linux", Architecture: "amd64"}
		arm64 := testutil.PushSinglePlatformImage(ctx, t, sourceRepo, "arm64")
		arm64.Platform = &ocispec.Platform{OS: "linux", Architecture: "arm64"}
		index := testutil.PushIndex(ctx, t, sourceRepo, []ocispec.Descriptor{amd64, arm64})
		require.NoError(t, sourceRepo.Tag(ctx, index, "multi"))

		cmd := newRegistryCommand()
		cmd.SetArgs([]string{"--insecure", "copy", fmt.Sprintf("%s/source:multi", sourceRegistry), fmt.Sprintf("%s/destination:all", destinationRegistry)})
		require.NoError(t, cmd.ExecuteContext(ctx))

		all, err := destinationRepo.Resolve(ctx, "all")
		require.NoError(t, err)
		require.Equal(t, index.Digest, all.Digest)
		require.Equal(t, index.MediaType, all.MediaType)
	})
}
