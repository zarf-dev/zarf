// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package archive

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"slices"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
)

const testLayer = "layer contents"

// buildTestImage pushes a one-layer image to a fresh OCI store and returns
// the store alongside the manifest descriptor Export takes.
func buildTestImage(ctx context.Context, t *testing.T) (*oci.Store, ocispec.Descriptor) {
	t.Helper()

	store, err := oci.New(t.TempDir())
	require.NoError(t, err)

	layer := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, []byte(testLayer))
	require.NoError(t, store.Push(ctx, layer, bytes.NewReader([]byte(testLayer))))

	configBytes, err := json.Marshal(ocispec.Image{
		Platform: ocispec.Platform{OS: "linux", Architecture: "amd64"},
		RootFS:   ocispec.RootFS{Type: "layers", DiffIDs: []digest.Digest{layer.Digest}},
	})
	require.NoError(t, err)
	config := content.NewDescriptorFromBytes(ocispec.MediaTypeImageConfig, configBytes)
	require.NoError(t, store.Push(ctx, config, bytes.NewReader(configBytes)))

	manifest, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, ocispec.MediaTypeImageManifest,
		oras.PackManifestOptions{Layers: []ocispec.Descriptor{layer}, ConfigDescriptor: &config})
	require.NoError(t, err)

	return store, manifest
}

// readArchive returns every entry of a tar in the order it was written.
func readArchive(t *testing.T, r io.Reader) ([]string, map[string][]byte) {
	t.Helper()

	var names []string
	contents := map[string][]byte{}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		names = append(names, hdr.Name)
		body, err := io.ReadAll(tr)
		require.NoError(t, err)
		contents[hdr.Name] = body
	}
	return names, contents
}

func TestExport(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	store, manifest := buildTestImage(ctx, t)

	var buf bytes.Buffer
	require.NoError(t, Export(ctx, store, manifest, "zarf.internal/docs:local", &buf))

	names, contents := readArchive(t, &buf)

	require.True(t, slices.IsSorted(names), "entries should be written in name order, got %v", names)
	require.Len(t, slices.Compact(slices.Clone(names)), len(names), "entries should not repeat a name")

	require.Contains(t, names, ocispec.ImageLayoutFile)
	require.Contains(t, names, ocispec.ImageIndexFile)
	require.Contains(t, names, "manifest.json")
	require.Contains(t, names, ocispec.ImageBlobsDir+"/")
	require.Contains(t, names, path.Join(ocispec.ImageBlobsDir, "sha256")+"/")

	var layout ocispec.ImageLayout
	require.NoError(t, json.Unmarshal(contents[ocispec.ImageLayoutFile], &layout))
	require.Equal(t, ocispec.ImageLayoutVersion, layout.Version)

	// Every blob is filed under the digest of its own bytes, so the archive
	// stays self-consistent for whoever extracts it.
	for _, name := range names {
		encoded, ok := cutBlobPath(name)
		if !ok {
			continue
		}
		require.Equal(t, encoded, digest.FromBytes(contents[name]).Encoded(), "blob %s does not hash to its path", name)
	}
}

// cutBlobPath reports whether name is a sha256 blob entry and returns the
// digest it is filed under.
func cutBlobPath(name string) (string, bool) {
	dir, encoded := path.Split(name)
	if dir != path.Join(ocispec.ImageBlobsDir, "sha256")+"/" || encoded == "" {
		return "", false
	}
	return encoded, true
}

func TestExportIndexNamesTheImage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	store, manifest := buildTestImage(ctx, t)

	var buf bytes.Buffer
	require.NoError(t, Export(ctx, store, manifest, "zarf.internal/docs:local", &buf))
	_, contents := readArchive(t, &buf)

	var index ocispec.Index
	require.NoError(t, json.Unmarshal(contents[ocispec.ImageIndexFile], &index))
	require.Len(t, index.Manifests, 1)

	exported := index.Manifests[0]
	require.Equal(t, manifest.Digest, exported.Digest)
	// Both annotations matter: src/pkg/images reads the first and falls back
	// to the second when pulling an image back out of an archive.
	require.Equal(t, "zarf.internal/docs:local", exported.Annotations[imageNameAnnotation])
	require.Equal(t, "local", exported.Annotations[ocispec.AnnotationRefName])
}

func TestExportDockerManifest(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	store, manifest := buildTestImage(ctx, t)

	var buf bytes.Buffer
	require.NoError(t, Export(ctx, store, manifest, "docker.io/library/nginx:1.29.2", &buf))
	names, contents := readArchive(t, &buf)

	var docker []dockerManifest
	require.NoError(t, json.Unmarshal(contents["manifest.json"], &docker))
	require.Len(t, docker, 1)

	// docker load reads the paths out of manifest.json, so each has to name an
	// entry that is actually in the archive.
	require.Contains(t, names, docker[0].Config)
	require.Len(t, docker[0].Layers, 1)
	require.Contains(t, names, docker[0].Layers[0])
	require.Equal(t, []byte(testLayer), contents[docker[0].Layers[0]])
	require.Equal(t, []string{"nginx:1.29.2"}, docker[0].RepoTags)
}

func TestExportRepoTagsAreFamiliar(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	testCases := []struct {
		name string
		ref  string
		want string
	}{
		{name: "docker hub library image loses its registry", ref: "docker.io/library/nginx:1.29.2", want: "nginx:1.29.2"},
		{name: "missing tag becomes latest", ref: "nginx", want: "nginx:latest"},
		{name: "private registry is kept whole", ref: "zarf.internal/docs:local", want: "zarf.internal/docs:local"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, manifest := buildTestImage(ctx, t)

			var buf bytes.Buffer
			require.NoError(t, Export(ctx, store, manifest, tc.ref, &buf))
			_, contents := readArchive(t, &buf)

			var docker []dockerManifest
			require.NoError(t, json.Unmarshal(contents["manifest.json"], &docker))
			require.Equal(t, []string{tc.want}, docker[0].RepoTags)
		})
	}
}

// TestExportRefNameFallsBackToTheWholeRef covers a reference with no tag to
// reduce to, where the OCI reference name is the reference itself.
func TestExportRefNameFallsBackToTheWholeRef(t *testing.T) {
	t.Parallel()

	require.Equal(t, "local", ociRefName("zarf.internal/docs:local"))
	require.Equal(t, "zarf.internal/docs", ociRefName("zarf.internal/docs"))
	require.Equal(t, "NOT A REF", ociRefName("NOT A REF"))
}

func TestExportUnparseableRef(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	store, manifest := buildTestImage(ctx, t)

	var buf bytes.Buffer
	require.ErrorContains(t, Export(ctx, store, manifest, "NOT A REF", &buf), "NOT A REF")
	require.Zero(t, buf.Len(), "nothing should be written for a reference that cannot be named")
}

func TestExportManifestNotInStore(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	store, _ := buildTestImage(ctx, t)
	missing := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, []byte("{}"))

	var buf bytes.Buffer
	require.Error(t, Export(ctx, store, missing, "zarf.internal/docs:local", &buf))
}
