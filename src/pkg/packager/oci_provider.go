// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// OCIProvider is a package provider for OCI registries.
type OCIProvider struct {
	source         string
	destinationDir string
	opts           *types.ZarfPackageOptions
	*oci.OrasRemote
}

// LoadPackage loads a package from an OCI registry.
func (op *OCIProvider) LoadPackage(optionalComponents []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = op.destinationDir
	layersToPull := []ocispec.Descriptor{}

	// only pull specified components and their images if optionalComponents AND --confirm are set
	if len(optionalComponents) > 0 && config.CommonOptions.Confirm {
		layers, err := op.LayersFromRequestedComponents(optionalComponents)
		if err != nil {
			return pkg, nil, fmt.Errorf("unable to get published component image layers: %s", err.Error())
		}
		layersToPull = append(layersToPull, layers...)
	}

	isPartial := true
	root, err := op.FetchRoot()
	if err != nil {
		return pkg, nil, err
	}
	if len(root.Layers) == len(layersToPull) {
		isPartial = false
	}

	pathsToCheck, err := op.PullPackage(op.destinationDir, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to pull the package: %w", err)
	}

	for _, path := range pathsToCheck {
		loaded[path] = filepath.Join(op.destinationDir, path)
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
		return pkg, nil, err
	}

	// always create and "load" components dir
	if _, ok := loaded[types.ZarfComponentsDir]; !ok {
		loaded[types.ZarfComponentsDir] = filepath.Join(op.destinationDir, types.ZarfComponentsDir)
		if err := utils.CreateDirectory(loaded[types.ZarfComponentsDir], 0755); err != nil {
			return pkg, nil, err
		}
	}

	// unpack component tarballs
	for _, component := range pkg.Components {
		tb := filepath.Join(op.destinationDir, types.ZarfComponentsDir, fmt.Sprintf("%s.tar", component.Name))
		if _, ok := loaded[tb]; ok {
			defer os.Remove(loaded[tb])
			defer delete(loaded, tb)
			if err = archiver.Unarchive(loaded[tb], loaded[types.ZarfComponentsDir]); err != nil {
				return pkg, nil, err
			}
		}
	}

	// unpack sboms.tar
	if _, ok := loaded[types.ZarfSBOMTar]; ok {
		loaded[types.ZarfSBOMDir] = filepath.Join(op.destinationDir, types.ZarfSBOMDir)
		if err = archiver.Unarchive(loaded[types.ZarfSBOMTar], loaded[types.ZarfSBOMDir]); err != nil {
			return pkg, nil, err
		}
	}

	return pkg, loaded, nil
}

// LoadPackageMetadata loads a package's metadata from an OCI registry.
func (op *OCIProvider) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = op.destinationDir
	var pathsToCheck []string

	metatdataDescriptors, err := op.PullPackageMetadata(op.destinationDir)
	if err != nil {
		return pkg, nil, err
	}

	for _, desc := range metatdataDescriptors {
		pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
	}

	if wantSBOM {
		sbomDescriptors, err := op.PullPackageSBOM(op.destinationDir)
		if err != nil {
			return pkg, nil, err
		}
		for _, desc := range sbomDescriptors {
			pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
		}
	}

	for _, path := range pathsToCheck {
		loaded[path] = filepath.Join(op.destinationDir, path)
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, true); err != nil {
		return pkg, nil, err
	}

	// unpack sboms.tar
	if _, ok := loaded[types.ZarfSBOMTar]; ok {
		loaded[types.ZarfSBOMDir] = filepath.Join(op.destinationDir, types.ZarfSBOMDir)
		if err = archiver.Unarchive(loaded[types.ZarfSBOMTar], loaded[types.ZarfSBOMDir]); err != nil {
			return pkg, nil, err
		}
	} else if wantSBOM {
		return pkg, nil, fmt.Errorf("package does not contain SBOMs")
	}

	return pkg, loaded, nil
}
