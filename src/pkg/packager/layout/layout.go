// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf packages.
package layout

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Constants used in the default package layout.
const (
	ZarfYAML  = "zarf.yaml"
	Signature = "zarf.yaml.sig"
	Checksums = "checksums.txt"

	ImagesDir     = "images"
	ComponentsDir = "components"
	ValuesDir     = "values"

	SBOMDir = "zarf-sbom"
	SBOMTar = "sboms.tar"

	IndexJSON = "index.json"
	OCILayout = "oci-layout"
)

var (
	// IndexPath is the path to the index.json file
	IndexPath = filepath.Join(ImagesDir, IndexJSON)
	// ImagesBlobsDir is the path to the directory containing the image blobs in the OCI package.
	ImagesBlobsDir = filepath.Join(ImagesDir, "blobs", "sha256")
	// OCILayoutPath is the path to the oci-layout file
	OCILayoutPath = filepath.Join(ImagesDir, OCILayout)
)

// ComponentDir is the type for the different directories in a component.
type ComponentDir string

// Different component directory types.
const (
	RepoComponentDir      ComponentDir = "repos"
	FilesComponentDir     ComponentDir = "files"
	ChartsComponentDir    ComponentDir = "charts"
	ManifestsComponentDir ComponentDir = "manifests"
	DataComponentDir      ComponentDir = "data"
	ValuesComponentDir    ComponentDir = "values"
)

// ContainsReservedFilename checks if a path uses a reserved Zarf filename
func ContainsReservedFilename(path string) error {
	base := filepath.Base(filepath.Clean(path))
	switch base {
	case ZarfYAML, Signature, Checksums:
		return fmt.Errorf("path cannot use reserved filename: %s", base)
	}
	return nil
}

// ContainsReservedPackageDir checks if a path traverses reserved package directories
func ContainsReservedPackageDir(path string) error {
	cleaned := filepath.Clean(path)
	parts := strings.Split(cleaned, string(filepath.Separator))

	reservedDirs := []string{ImagesDir, ComponentsDir, SBOMDir}
	for _, part := range parts {
		for _, reserved := range reservedDirs {
			if part == reserved {
				return fmt.Errorf("path cannot traverse reserved directory: %s", reserved)
			}
		}
	}
	return nil
}
