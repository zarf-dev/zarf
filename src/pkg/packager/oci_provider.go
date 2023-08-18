// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociProvider struct {
	src string
	dst types.PackagePathsMap
	*oci.OrasRemote
	signatureValidator
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

	_, err = op.PullPackage(op.dst.Base(), config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return nil, err
	}
	// TODO: checksum validation

	return pkg, utils.ReadYaml(op.dst[types.ZarfYAML], &pkg)
}

func (op *ociProvider) LoadPackageMetadata(wantSBOM bool) (pkg *types.ZarfPackage, err error) {
	_, err = op.PullPackageMetadata(op.dst.Base())
	if err != nil {
		return nil, err
	}
	if wantSBOM {
		_, err = op.PullPackageSBOM(op.dst.Base())
		if err != nil {
			return nil, err
		}
	}
	// TODO: checksum validation

	return pkg, utils.ReadYaml(op.dst[types.ZarfYAML], &pkg)
}
