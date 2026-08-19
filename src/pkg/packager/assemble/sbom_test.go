// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/anchore/syft/syft/format/syftjson/model"
	"github.com/anchore/syft/syft/pkg"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
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

// findArtifact looks a package up by name in a syft document. Decoding into
// syft's own model rather than a local projection means a change to the
// syft-json shape surfaces here, which is the point: the document is a zarf
// output, so its schema is part of what these tests are guarding.
func findArtifact(doc model.Document, name string) (version string, pkgType pkg.Type, ok bool) {
	for _, a := range doc.Artifacts {
		if a.Name == name {
			return a.Version, a.Type, true
		}
	}
	return "", "", false
}

func layerFromFiles(t *testing.T, files map[string]string) v1.Layer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	})
	require.NoError(t, err)
	return layer
}

func TestCreateImageSBOMContents(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	apkInstalledDB := `P:musl
V:1.2.4-r2
A:aarch64
T:the musl c library
L:MIT

P:zlib
V:1.3-r2
A:aarch64
T:A compression library
L:Zlib
`
	layer := layerFromFiles(t, map[string]string{
		"lib/apk/db/installed": apkInstalledDB,
	})
	img, err := mutate.AppendLayers(empty.Image, layer)
	require.NoError(t, err)

	outputPath := t.TempDir()
	b, err := createImageSBOM(ctx, t.TempDir(), outputPath, img, "docker.io/foo/bar:latest")
	require.NoError(t, err)

	var doc model.Document
	require.NoError(t, json.Unmarshal(b, &doc))

	version, pkgType, ok := findArtifact(doc, "musl")
	require.True(t, ok, "expected musl package in image SBOM artifacts")
	require.Equal(t, "1.2.4-r2", version)
	require.Equal(t, pkg.ApkPkg, pkgType)

	version, _, ok = findArtifact(doc, "zlib")
	require.True(t, ok, "expected zlib package in image SBOM artifacts")
	require.Equal(t, "1.3-r2", version)

	require.Equal(t, "zarf", doc.Descriptor.Name)
	require.NotEmpty(t, doc.Source.Type)
	require.NotEmpty(t, doc.Schema.URL)
	require.NotEmpty(t, doc.ArtifactRelationships)
}

func TestCreateFileSBOMContents(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	component := v1alpha1.ZarfComponent{
		Name: "test-component",
		Files: []v1alpha1.ZarfFile{
			{Target: "requirements.txt"},
		},
	}

	// Lay out the component tar the way assemble produces it:
	// <component>/files/<idx>/<basename(target)>
	buildPath := t.TempDir()
	componentsDir := filepath.Join(buildPath, string(layout.ComponentsDir))
	require.NoError(t, os.MkdirAll(componentsDir, 0o755))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	// tar entry names are always slash-separated, so filepath.Join would emit
	// backslashes on Windows and the extractor would reject the entry.
	entry := path.Join(component.Name, string(layout.FilesComponentDir), filepath.ToSlash(layout.ComponentFileRelPath(0, "requirements.txt")))
	content := "flask==2.0.1\nrequests==2.31.0\n"
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: entry, Mode: 0o644, Size: int64(len(content))}))
	_, err := tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, os.WriteFile(filepath.Join(componentsDir, component.Name+".tar"), buf.Bytes(), 0o644))

	outputPath := t.TempDir()
	b, err := createFileSBOM(ctx, component, outputPath, buildPath)
	require.NoError(t, err)

	var doc model.Document
	require.NoError(t, json.Unmarshal(b, &doc))

	version, pkgType, ok := findArtifact(doc, "flask")
	require.True(t, ok, "expected flask package in file SBOM artifacts")
	require.Equal(t, "2.0.1", version)
	require.Equal(t, pkg.PythonPkg, pkgType)

	_, _, ok = findArtifact(doc, "requests")
	require.True(t, ok, "expected requests package in file SBOM artifacts")

	require.Equal(t, "zarf", doc.Descriptor.Name)
	require.NotEmpty(t, doc.ArtifactRelationships)

	// The returned bytes are also written to zarf-component-<name>.json.
	fileContent, err := os.ReadFile(filepath.Join(outputPath, "zarf-component-test-component.json"))
	require.NoError(t, err)
	require.Equal(t, fileContent, b)
}
