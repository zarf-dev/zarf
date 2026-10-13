// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf packages.
package layout

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Constants used in the default package layout.
const (
	ZarfYAML          = "zarf.yaml"
	ZarfGeneratedYAML = "zarf.gen.yaml"
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

// ValidatePathComponent ensures a package-derived value is safe to use in an internal path.
func ValidatePathComponent(value string) error {
	if value == ".." {
		return fmt.Errorf("%q must not be a traversal path", value)
	}
	if strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("%q must not contain path separators", value)
	}
	return nil
}

// PathWithinDirectory resolves filename under directory and rejects paths that escape it.
func PathWithinDirectory(directory, filename string) (string, error) {
	destination := filepath.Join(directory, filename)
	relative, err := filepath.Rel(directory, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes %q", destination, directory)
	}
	return destination, nil
}

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
	// replace Windows \ paths with / so that *nix-created packages resolve the same Windows target filename
	target = strings.ReplaceAll(target, `\`, "/")
	return filepath.Join(strconv.Itoa(idx), path.Base(target))
}

// chartStem is the name both of a chart's packaged artifacts are built from:
// "<name>" when the version is empty, otherwise "<name>-<version>".
func chartStem(name, version string) string {
	if version == "" {
		return name
	}
	return name + "-" + version
}

// ChartArchiveName returns the file name of a chart's packaged tarball, within a
// component's charts directory.
func ChartArchiveName(name, version string) string {
	return chartStem(name, version) + ".tgz"
}

// ChartValuesFileName returns the file name of the idx-th values file of the named
// chart, within a component's values directory.
func ChartValuesFileName(name, version string, idx int) string {
	return chartStem(name, version) + "-" + strconv.Itoa(idx)
}

// ChartPaths resolves the on-disk locations of a chart's packaged artifacts
// within a component's charts and values directories.
type ChartPaths struct {
	// ChartsDir is the directory holding chart tarballs.
	ChartsDir string
	// ValuesDir is the directory holding chart values files.
	ValuesDir string
}

// Archive returns the full path to the named chart's packaged tarball.
func (p ChartPaths) Archive(name, version string) string {
	return filepath.Join(p.ChartsDir, ChartArchiveName(name, version))
}

// ValuesFile returns the full path to the idx-th values file for the named chart.
func (p ChartPaths) ValuesFile(name, version string, idx int) string {
	return filepath.Join(p.ValuesDir, ChartValuesFileName(name, version, idx))
}

// ArchivePath returns the contained path for a chart archive.
func (p ChartPaths) ArchivePath(name, version string) (string, error) {
	if err := ValidatePathComponent(name); err != nil {
		return "", fmt.Errorf("chart name validation failed: %w", err)
	}
	if err := ValidatePathComponent(version); err != nil {
		return "", fmt.Errorf("chart version validation failed: %w", err)
	}
	archivePath, err := PathWithinDirectory(p.ChartsDir, ChartArchiveName(name, version))
	if err != nil {
		return "", fmt.Errorf("chart archive validation failed: %w", err)
	}
	return archivePath, nil
}

// ValuesFilePath returns the contained path for a packaged chart values file.
func (p ChartPaths) ValuesFilePath(name, version string, idx int) (string, error) {
	if err := ValidatePathComponent(name); err != nil {
		return "", fmt.Errorf("chart name validation failed: %w", err)
	}
	if err := ValidatePathComponent(version); err != nil {
		return "", fmt.Errorf("chart version validation failed: %w", err)
	}
	valuesPath, err := PathWithinDirectory(p.ValuesDir, ChartValuesFileName(name, version, idx))
	if err != nil {
		return "", fmt.Errorf("chart values validation failed: %w", err)
	}
	return valuesPath, nil
}
