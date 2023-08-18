// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociProvider struct {
	src string
	dst types.PackagePathsMap
	*oci.OrasRemote
}

func (op *ociProvider) LoadPackage(optionalComponents []string) (pkg *types.ZarfPackage, err error) {
	layersToPull := []ocispec.Descriptor{}

	// only pull specified components and their images if optionalComponents AND --confirm are set
	if len(optionalComponents) > 0 && config.CommonOptions.Confirm {
		layers, err := op.LayersFromRequestedComponents(optionalComponents)
		if err != nil {
			return nil, fmt.Errorf("unable to get published component image layers: %s", err.Error())
		}
		layersToPull = append(layersToPull, layers...)
	}

	if err := utils.ReadYaml(op.dst[types.ZarfYAML], &pkg); err != nil {
		return nil, err
	}

	if err := validate.PackageIntegrity(op.dst, nil, pkg.Metadata.AggregateChecksum); err != nil {
		return nil, err
	}

	return pkg, nil
}

func (op *ociProvider) LoadPackageMetadata(wantSBOM bool) (pkg *types.ZarfPackage, err error) {
	var pathsToCheck []string

	metatdataDescriptors, err := op.PullPackageMetadata(op.dst.Base())
	if err != nil {
		return nil, err
	}

	for _, desc := range metatdataDescriptors {
		pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
	}

	if wantSBOM {
		sbomDescriptors, err := op.PullPackageSBOM(op.dst.Base())
		if err != nil {
			return nil, err
		}
		for _, desc := range sbomDescriptors {
			pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
		}
	}

	if err := utils.ReadYaml(op.dst[types.ZarfYAML], &pkg); err != nil {
		return nil, err
	}

	if err := validate.PackageIntegrity(op.dst, pathsToCheck, pkg.Metadata.AggregateChecksum); err != nil {
		return nil, err
	}

	return pkg, nil
}
