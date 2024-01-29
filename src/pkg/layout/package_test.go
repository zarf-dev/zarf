// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"runtime"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestPackageFiles(t *testing.T) {
	pp := New("test")

	raw := &PackagePaths{
		Base:      "test",
		ZarfYAML:  normalizePath("test/zarf.yaml"),
		Checksums: normalizePath("test/checksums.txt"),
		Components: Components{
			Base: normalizePath("test/components"),
		},
	}

	require.Equal(t, raw, pp)

	files := pp.Files()

	expected := map[string]string{
		"zarf.yaml":     normalizePath("test/zarf.yaml"),
		"checksums.txt": normalizePath("test/checksums.txt"),
	}

	require.Equal(t, expected, files)

	pp = pp.addSignature("")

	files = pp.Files()

	// addSignature will only add the signature if it is not empty
	require.Equal(t, expected, files)

	pp = pp.addSignature("key.priv")

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":     normalizePath("test/zarf.yaml"),
		"checksums.txt": normalizePath("test/checksums.txt"),
		"zarf.yaml.sig": normalizePath("test/zarf.yaml.sig"),
	}

	require.Equal(t, expected, files)
	pp = pp.AddImages()

	files = pp.Files()

	// Note that the map key will always be the forward "Slash" (/) version of the file path (never \)
	expected = map[string]string{
		"zarf.yaml":         normalizePath("test/zarf.yaml"),
		"checksums.txt":     normalizePath("test/checksums.txt"),
		"zarf.yaml.sig":     normalizePath("test/zarf.yaml.sig"),
		"images/index.json": normalizePath("test/images/index.json"),
		"images/oci-layout": normalizePath("test/images/oci-layout"),
	}

	require.Equal(t, expected, files)

	pp = pp.AddSBOMs()

	files = pp.Files()

	// AddSBOMs adds the SBOMs directory, and files will only cares about files
	require.Equal(t, expected, files)

	paths := []string{
		"zarf.yaml",
		"checksums.txt",
		"sboms.tar",
		normalizePath("components/c1.tar"),
		normalizePath("images/index.json"),
		normalizePath("images/oci-layout"),
		normalizePath("images/blobs/sha256/" + strings.Repeat("1", 64)),
	}

	pp = New("test")

	pp.SetFromPaths(paths)

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":         normalizePath("test/zarf.yaml"),
		"checksums.txt":     normalizePath("test/checksums.txt"),
		"sboms.tar":         normalizePath("test/sboms.tar"),
		"components/c1.tar": normalizePath("test/components/c1.tar"),
		"images/index.json": normalizePath("test/images/index.json"),
		"images/oci-layout": normalizePath("test/images/oci-layout"),
		"images/blobs/sha256/" + strings.Repeat("1", 64): normalizePath("test/images/blobs/sha256/" + strings.Repeat("1", 64)),
	}

	require.Len(t, pp.Images.Blobs, 1)

	require.Equal(t, expected, files)

	descs := []ocispec.Descriptor{
		{
			Annotations: map[string]string{
				ocispec.AnnotationTitle: "components/c2.tar",
			},
		},
		{
			Annotations: map[string]string{
				ocispec.AnnotationTitle: "images/blobs/sha256/" + strings.Repeat("2", 64),
			},
		},
	}

	pp.SetFromLayers(descs)

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":         normalizePath("test/zarf.yaml"),
		"checksums.txt":     normalizePath("test/checksums.txt"),
		"sboms.tar":         normalizePath("test/sboms.tar"),
		"components/c1.tar": normalizePath("test/components/c1.tar"),
		"components/c2.tar": normalizePath("test/components/c2.tar"),
		"images/index.json": normalizePath("test/images/index.json"),
		"images/oci-layout": normalizePath("test/images/oci-layout"),
		"images/blobs/sha256/" + strings.Repeat("1", 64): normalizePath("test/images/blobs/sha256/" + strings.Repeat("1", 64)),
		"images/blobs/sha256/" + strings.Repeat("2", 64): normalizePath("test/images/blobs/sha256/" + strings.Repeat("2", 64)),
	}

	require.Equal(t, expected, files)
}

// normalizePath ensures that the filepaths being generated are normalized to the host OS.
func normalizePath(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}

	return strings.ReplaceAll(path, "/", "\\")
}
