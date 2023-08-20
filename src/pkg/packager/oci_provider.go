// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type OCIProvider struct {
	source         string
	destinationDir string
	opts           *types.ZarfPackageOptions
	*oci.OrasRemote
}

func (op *OCIProvider) LoadPackage(optionalComponents []string) (pkg *types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded["base"] = op.destinationDir
	layersToPull := []ocispec.Descriptor{}

	// only pull specified components and their images if optionalComponents AND --confirm are set
	if len(optionalComponents) > 0 && config.CommonOptions.Confirm {
		layers, err := op.LayersFromRequestedComponents(optionalComponents)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get published component image layers: %s", err.Error())
		}
		layersToPull = append(layersToPull, layers...)
	}

	pathsToCheck, err := op.PullPackage(op.destinationDir, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to pull the package: %w", err)
	}

	for _, path := range pathsToCheck {
		loaded[path] = filepath.Join(op.destinationDir, path)
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return nil, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pathsToCheck, pkg.Metadata.AggregateChecksum); err != nil {
		return nil, nil, err
	}

	return pkg, loaded, nil
}

func (op *OCIProvider) LoadPackageMetadata(wantSBOM bool) (pkg *types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded["base"] = op.destinationDir
	var pathsToCheck []string

	metatdataDescriptors, err := op.PullPackageMetadata(op.destinationDir)
	if err != nil {
		return nil, nil, err
	}

	for _, desc := range metatdataDescriptors {
		pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
	}

	if wantSBOM {
		sbomDescriptors, err := op.PullPackageSBOM(op.destinationDir)
		if err != nil {
			return nil, nil, err
		}
		for _, desc := range sbomDescriptors {
			pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
		}
	}

	for _, path := range pathsToCheck {
		loaded[path] = filepath.Join(op.destinationDir, path)
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return nil, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pathsToCheck, pkg.Metadata.AggregateChecksum); err != nil {
		return nil, nil, err
	}

	return pkg, loaded, nil
}
