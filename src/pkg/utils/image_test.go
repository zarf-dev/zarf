// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func TestLoadOCIImagePlatformsSingleArch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)

	img, err := random.Image(256, 1)
	require.NoError(t, err)

	ref := "example.com/foo/bar:1.0.0"
	require.NoError(t, path.AppendImage(img, layout.WithAnnotations(map[string]string{
		ocispec.AnnotationBaseImageName: ref,
		ocispec.AnnotationRefName:       ref,
	})))

	refInfo, err := transform.ParseImageRef(ref)
	require.NoError(t, err)

	images, err := LoadOCIImagePlatforms(dir, refInfo)
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Nil(t, images[0].Platform)
	require.NotNil(t, images[0].Image)
}

func TestLoadOCIImagePlatformsMultiArch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)

	amd64, err := random.Image(256, 1)
	require.NoError(t, err)
	arm64, err := random.Image(256, 1)
	require.NoError(t, err)
	unknown, err := random.Image(256, 1)
	require.NoError(t, err)

	idx := mutate.AppendManifests(empty.Index,
		mutate.IndexAddendum{
			Add: amd64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "linux", Architecture: "amd64"},
			},
		},
		mutate.IndexAddendum{
			Add: arm64,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "linux", Architecture: "arm64"},
			},
		},
		mutate.IndexAddendum{
			Add: unknown,
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: "unknown", Architecture: "unknown"},
			},
		},
	)

	ref := "example.com/foo/multi:1.0.0"
	require.NoError(t, path.AppendIndex(idx, layout.WithAnnotations(map[string]string{
		ocispec.AnnotationBaseImageName: ref,
		ocispec.AnnotationRefName:       ref,
	})))

	refInfo, err := transform.ParseImageRef(ref)
	require.NoError(t, err)

	images, err := LoadOCIImagePlatforms(dir, refInfo)
	require.NoError(t, err)
	require.Len(t, images, 2, "unknown/unknown manifests must be skipped")

	var platforms []string
	for _, pi := range images {
		require.NotNil(t, pi.Platform)
		require.NotNil(t, pi.Image)
		platforms = append(platforms, pi.Platform.OS+"/"+pi.Platform.Architecture)
	}
	require.ElementsMatch(t, []string{"linux/amd64", "linux/arm64"}, platforms)
}

func TestLoadOCIImagePlatformsNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)

	refInfo, err := transform.ParseImageRef("example.com/foo/missing:1.0.0")
	require.NoError(t, err)

	_, err = LoadOCIImagePlatforms(dir, refInfo)
	require.ErrorContains(t, err, "unable to find image")
}
