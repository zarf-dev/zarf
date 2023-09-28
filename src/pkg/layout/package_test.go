// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestPackage_Files(t *testing.T) {
	pp := New("test")

	raw := &PackagePaths{
		Base:      "test",
		ZarfYAML:  "test/zarf.yaml",
		Checksums: "test/checksums.txt",
		Components: Components{
			Base: "test/components",
		},
	}

	require.Equal(t, raw, pp)

	files := pp.Files()

	expected := map[string]string{
		"zarf.yaml":     "test/zarf.yaml",
		"checksums.txt": "test/checksums.txt",
	}

	require.Equal(t, expected, files)

	pp = pp.AddSignature("")

	files = pp.Files()

	// AddSignature will only add the signature if it is not empty
	require.Equal(t, expected, files)

	pp = pp.AddSignature("key.priv")

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":     "test/zarf.yaml",
		"checksums.txt": "test/checksums.txt",
		"zarf.yaml.sig": "test/zarf.yaml.sig",
	}

	require.Equal(t, expected, files)

	pp = pp.AddImages()

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":         "test/zarf.yaml",
		"checksums.txt":     "test/checksums.txt",
		"zarf.yaml.sig":     "test/zarf.yaml.sig",
		"images/index.json": "test/images/index.json",
		"images/oci-layout": "test/images/oci-layout",
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
		"components/c1.tar",
		"images/index.json",
		"images/oci-layout",
		"images/blobs/sha256/" + strings.Repeat("1", 64),
	}

	pp = New("test")

	pp.SetFromPaths(paths)

	files = pp.Files()

	expected = map[string]string{
		"zarf.yaml":         "test/zarf.yaml",
		"checksums.txt":     "test/checksums.txt",
		"sboms.tar":         "test/sboms.tar",
		"components/c1.tar": "test/components/c1.tar",
		"images/index.json": "test/images/index.json",
		"images/oci-layout": "test/images/oci-layout",
		"images/blobs/sha256/" + strings.Repeat("1", 64): "test/images/blobs/sha256/" + strings.Repeat("1", 64),
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
		"zarf.yaml":         "test/zarf.yaml",
		"checksums.txt":     "test/checksums.txt",
		"sboms.tar":         "test/sboms.tar",
		"components/c1.tar": "test/components/c1.tar",
		"components/c2.tar": "test/components/c2.tar",
		"images/index.json": "test/images/index.json",
		"images/oci-layout": "test/images/oci-layout",
		"images/blobs/sha256/" + strings.Repeat("1", 64): "test/images/blobs/sha256/" + strings.Repeat("1", 64),
		"images/blobs/sha256/" + strings.Repeat("2", 64): "test/images/blobs/sha256/" + strings.Repeat("2", 64),
	}

	require.Equal(t, expected, files)
}
