// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package providers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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

	message.Debugf("Loading package from %q", op.source)
	message.Debugf("Loaded package base directory: %q", op.destinationDir)

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

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, op.opts.PublicKeyPath); err != nil {
		return pkg, nil, err
	}

	// always create and "load" components dir
	if _, ok := loaded[types.ComponentsDir]; !ok {
		message.Debugf("Creating %q dir", types.ComponentsDir)
		loaded[types.ComponentsDir] = filepath.Join(op.destinationDir, types.ComponentsDir)
		if err := utils.CreateDirectory(loaded[types.ComponentsDir], 0755); err != nil {
			return pkg, nil, err
		}
	}

	// unpack component tarballs
	for _, component := range pkg.Components {
		tb := filepath.Join(types.ComponentsDir, fmt.Sprintf("%s.tar", component.Name))
		if _, ok := loaded[tb]; ok {
			message.Debugf("Unarchiving %q", tb)
			defer os.Remove(loaded[tb])
			defer delete(loaded, tb)
			if err = archiver.Unarchive(loaded[tb], loaded[types.ComponentsDir]); err != nil {
				return pkg, nil, err
			}
		}

		// also "load" the images dir if any component has images
		if _, ok := loaded[types.ImagesDir]; !ok && len(component.Images) > 0 {
			message.Debugf("Creating %q dir", types.ImagesDir)
			loaded[types.ImagesDir] = filepath.Join(op.destinationDir, types.ImagesDir)
			if err := utils.CreateDirectory(loaded[types.ImagesDir], 0755); err != nil {
				return pkg, nil, err
			}
		}
	}

	// unpack sboms.tar
	if _, ok := loaded[types.SBOMTar]; ok {
		message.Debugf("Unarchiving %q", types.SBOMTar)
		loaded[types.SBOMDir] = filepath.Join(op.destinationDir, types.SBOMDir)
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
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

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, true); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, op.opts.PublicKeyPath); err != nil {
		if errors.Is(err, ErrPkgSigButNoKey) {
			message.Warn("The package was signed but no public key was provided, skipping signature validation")
		} else {
			return pkg, nil, err
		}
	}

	// unpack sboms.tar
	if _, ok := loaded[types.SBOMTar]; ok {
		loaded[types.SBOMDir] = filepath.Join(op.destinationDir, types.SBOMDir)
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
			return pkg, nil, err
		}
	} else if wantSBOM {
		return pkg, nil, fmt.Errorf("package does not contain SBOMs")
	}

	return pkg, loaded, nil
}
