// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

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
