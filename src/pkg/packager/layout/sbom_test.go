// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestCreateImageSBOM(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	outputPath := t.TempDir()
	img := empty.Image
	b, err := createImageSBOM(ctx, t.TempDir(), outputPath, img, "docker.io/foo/bar:latest")
	require.NoError(t, err)
	require.NotEmpty(t, b)

	fileContent, err := os.ReadFile(filepath.Join(outputPath, "docker.io_foo_bar_latest.json"))
	require.NoError(t, err)
	require.Equal(t, fileContent, b)
}

func TestCreateImageSBOMNonExistentCachePath(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	outputPath := t.TempDir()
	// Cache path that doesn't exist yet
	cachePath := filepath.Join(t.TempDir(), "non-existent-cache")
	img := empty.Image
	b, err := createImageSBOM(ctx, cachePath, outputPath, img, "docker.io/foo/bar:latest")
	require.NoError(t, err)
	require.NotEmpty(t, b)
}

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

	images, err := loadOCIImagePlatforms(dir, refInfo)
	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Nil(t, images[0].platform)
	require.NotNil(t, images[0].image)
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
	)

	ref := "example.com/foo/multi:1.0.0"
	require.NoError(t, path.AppendIndex(idx, layout.WithAnnotations(map[string]string{
		ocispec.AnnotationBaseImageName: ref,
		ocispec.AnnotationRefName:       ref,
	})))

	refInfo, err := transform.ParseImageRef(ref)
	require.NoError(t, err)

	images, err := loadOCIImagePlatforms(dir, refInfo)
	require.NoError(t, err)
	require.Len(t, images, 2)

	var platforms []string
	for _, pi := range images {
		require.NotNil(t, pi.platform)
		require.NotNil(t, pi.image)
		platforms = append(platforms, pi.platform.OS+"/"+pi.platform.Architecture)
	}
	require.ElementsMatch(t, []string{"linux/amd64", "linux/arm64"}, platforms)
}

func TestLoadOCIImagePlatformsNestedIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path, err := layout.Write(dir, empty.Index)
	require.NoError(t, err)

	amd64, err := random.Image(256, 1)
	require.NoError(t, err)
	arm64, err := random.Image(256, 1)
	require.NoError(t, err)

	inner := mutate.AppendManifests(empty.Index,
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
	)
	outer := mutate.AppendManifests(empty.Index, mutate.IndexAddendum{Add: inner})

	ref := "example.com/foo/nested:1.0.0"
	require.NoError(t, path.AppendIndex(outer, layout.WithAnnotations(map[string]string{
		ocispec.AnnotationBaseImageName: ref,
		ocispec.AnnotationRefName:       ref,
	})))

	refInfo, err := transform.ParseImageRef(ref)
	require.NoError(t, err)

	images, err := loadOCIImagePlatforms(dir, refInfo)
	require.NoError(t, err)
	require.Len(t, images, 2, "nested platform manifests must be found")

	var platforms []string
	for _, pi := range images {
		require.NotNil(t, pi.platform)
		platforms = append(platforms, pi.platform.OS+"/"+pi.platform.Architecture)
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

	_, err = loadOCIImagePlatforms(dir, refInfo)
	require.ErrorContains(t, err, "unable to find image")
}
