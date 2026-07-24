// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf packages.
package layout

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// Constants used in the default package layout.
const (
	ZarfYAML = "zarf.yaml"
	// Deprecated: legacy signature format superseded by Bundle (zarf.bundle.sig) since v0.71.0 and no longer produced as of v0.81.0.
	// This field is retained to ensure backwards compatibility with verification of older packages.
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

// ChartArchiveName returns the base file name for a chart's packaged tarball:
// "<name>" when the version is empty, otherwise "<name>-<version>".
func ChartArchiveName(chart v1alpha1.ZarfChart) string {
	if chart.Version == "" {
		return chart.Name
	}
	return chart.Name + "-" + chart.Version
}

// ChartValuesFileName returns the base file name for the idx-th values file of a
// chart, as stored within a component's values directory.
func ChartValuesFileName(chart v1alpha1.ZarfChart, idx int) string {
	return ChartArchiveName(chart) + "-" + strconv.Itoa(idx)
}

// ChartPaths resolves the on-disk locations of a chart's packaged artifacts
// within a component's charts and values directories. It satisfies the path seam
// the `helm` package depends on, so `helm` receives resolved paths rather than
// re-deriving the package layout convention itself.
type ChartPaths struct {
	// ChartsDir is the directory holding chart tarballs.
	ChartsDir string
	// ValuesDir is the directory holding chart values files.
	ValuesDir string
}

// Archive returns the full path to the chart's packaged tarball.
func (p ChartPaths) Archive(chart v1alpha1.ZarfChart) string {
	return filepath.Join(p.ChartsDir, ChartArchiveName(chart)) + ".tgz"
}

// ValuesFile returns the full path to the idx-th values file for the chart.
func (p ChartPaths) ValuesFile(chart v1alpha1.ZarfChart, idx int) string {
	return filepath.Join(p.ValuesDir, ChartValuesFileName(chart, idx))
}
