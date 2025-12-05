// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf packages.
package layout

import (
	"path/filepath"
)

// Constants used in the default package layout.
const (
	ZarfYAML     = "zarf.yaml"
	Signature    = "zarf.yaml.sig"
	Checksums    = "checksums.txt"
	ValuesYAML   = "values.yaml"
	ValuesSchema = "values.schema.json"

	ImagesDir        = "images"
	ComponentsDir    = "components"
	ValuesDir        = "values"
	DocumentationDir = "documentation"

	SBOMDir = "zarf-sbom"
	SBOMTar = "sboms.tar"

	DocumentationTar = "documentation.tar"

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
