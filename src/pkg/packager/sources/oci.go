// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// OCISource is a package source for OCI registries.
type OCISource struct {
	Destination types.PackagePathsMap
	*types.ZarfPackageOptions
	*oci.OrasRemote
}

// LoadPackage loads a package from an OCI registry.
func (s *OCISource) LoadPackage() (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = s.Destination
	layersToPull := []ocispec.Descriptor{}

	message.Debugf("Loading package from %q", s.PackageSource)
	message.Debugf("Loaded package base directory: %q", loaded.Base())

	optionalComponents := helpers.StringToSlice(s.OptionalComponents)

	// only pull specified components and their images if optionalComponents AND --confirm are set
	if len(optionalComponents) > 0 && config.CommonOptions.Confirm {
		layers, err := s.LayersFromRequestedComponents(optionalComponents)
		if err != nil {
			return pkg, nil, fmt.Errorf("unable to get published component image layers: %s", err.Error())
		}
		layersToPull = append(layersToPull, layers...)
	}

	isPartial := true
	root, err := s.FetchRoot()
	if err != nil {
		return pkg, nil, err
	}
	if len(root.Layers) == len(layersToPull) {
		isPartial = false
	}

	pathsToCheck, err := s.PullPackage(loaded.Base(), config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to pull the package: %w", err)
	}

	for _, path := range pathsToCheck {
		if err := loaded.SetDefaultRelative(path); err != nil {
			return pkg, nil, err
		}
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, s.PublicKeyPath); err != nil {
		return pkg, nil, err
	}

	if err := LoadComponents(&pkg, loaded); err != nil {
		return pkg, nil, err
	}

	if err := LoadSBOMs(loaded); err != nil {
		return pkg, nil, err
	}

	return pkg, loaded, nil
}

// LoadPackageMetadata loads a package's metadata from an OCI registry.
func (s *OCISource) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = s.Destination
	var pathsToCheck []string

	metatdataDescriptors, err := s.PullPackageMetadata(loaded.Base())
	if err != nil {
		return pkg, nil, err
	}
	for _, desc := range metatdataDescriptors {
		pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
	}

	if wantSBOM {
		sbomDescriptors, err := s.PullPackageSBOM(loaded.Base())
		if err != nil {
			return pkg, nil, err
		}
		for _, desc := range sbomDescriptors {
			pathsToCheck = append(pathsToCheck, desc.Annotations[ocispec.AnnotationTitle])
		}
	}

	for _, path := range pathsToCheck {
		if err := loaded.SetDefaultRelative(path); err != nil {
			return pkg, nil, err
		}
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, true); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, s.PublicKeyPath); err != nil {
		if errors.Is(err, ErrPkgSigButNoKey) {
			message.Warn("The package was signed but no public key was provided, skipping signature validation")
		} else {
			return pkg, nil, err
		}
	}

	// unpack sboms.tar
	if loaded.KeyExists(types.SBOMTar) {
		message.Debugf("Unarchiving %q", types.SBOMTar)
		defer os.Remove(loaded[types.SBOMTar])
		defer loaded.Unset(types.SBOMTar)
		if err := loaded.SetDefaultRelative(types.SBOMDir); err != nil {
			return pkg, nil, err
		}
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
			return pkg, nil, err
		}
	} else if wantSBOM {
		return pkg, nil, fmt.Errorf("package does not contain SBOMs")
	}

	return pkg, loaded, nil
}

// Collect pulls a package from an OCI registry and writes it to a tarball.
func (s *OCISource) Collect(dstTarball string) error {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	_, err = s.PullPackage(tmp, config.CommonOptions.OCIConcurrency)
	if err != nil {
		return err
	}

	allTheLayers, err := filepath.Glob(filepath.Join(tmp, "*"))
	if err != nil {
		return err
	}

	_ = os.Remove(dstTarball)

	return archiver.Archive(allTheLayers, dstTarball)
}
