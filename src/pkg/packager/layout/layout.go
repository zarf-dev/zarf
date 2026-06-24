// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf packages.
package layout

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// Constants used in the default package layout.
const (
	ZarfYAML     = "zarf.yaml"
	Signature    = "zarf.yaml.sig"
	Bundle       = "zarf.bundle.sig"
	Checksums    = "checksums.txt"
	ValuesYAML   = "values.yaml"
	ValuesSchema = "values.schema.json"

	ImagesDir     = "images"
	ComponentsDir = "components"

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

// ManifestFileName returns the file name, within a component's manifests directory, that stores the
// idx-th file of the named manifest.
func ManifestFileName(manifestName string, idx int) string {
	return fmt.Sprintf("%s-%d.yaml", manifestName, idx)
}

// KustomizationFileName returns the file name, within a component's manifests directory, that stores
// the idx-th rendered kustomization of the named manifest.
func KustomizationFileName(manifestName string, idx int) string {
	return fmt.Sprintf("kustomization-%s-%d.yaml", manifestName, idx)
}

// ComponentFileRelPath returns the path, relative to a component's files directory, where the idx-th
// file's contents are stored.
func ComponentFileRelPath(idx int, target string) string {
	return filepath.Join(strconv.Itoa(idx), filepath.Base(target))
}
