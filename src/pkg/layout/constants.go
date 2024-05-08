// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import "path/filepath"

// Constants used in the default package layout.
const (
	TempDir           = "temp"
	FilesDir          = "files"
	ChartsDir         = "charts"
	ReposDir          = "repos"
	ManifestsDir      = "manifests"
	DataInjectionsDir = "data"
	ValuesDir         = "values"

	ZarfYAML  = "zarf.yaml"
	Signature = "zarf.yaml.sig"
	Checksums = "checksums.txt"

	ImagesDir     = "images"
	ComponentsDir = "components"

	SBOMDir = "zarf-sbom"
	SBOMTar = "sboms.tar"

	IndexJSON = "index.json"
	OCILayout = "oci-layout"

	SeedImagesDir        = "seed-images"
	InjectorBinary       = "zarf-injector"
	InjectorPayloadTarGz = "payload.tgz"
)

var (
	// IndexPath is the path to the index.json file
	IndexPath = filepath.Join(ImagesDir, IndexJSON)
	// ImagesBlobsDir is the path to the directory containing the image blobs in the OCI package.
	ImagesBlobsDir = filepath.Join(ImagesDir, "blobs", "sha256")
	// OCILayoutPath is the path to the oci-layout file
	OCILayoutPath = filepath.Join(ImagesDir, OCILayout)
)
