// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/ocizarf"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// veryify that OCISource implements PackageSource
	_ PackageSource = (*OCISource)(nil)
)

// OCISource is a package source for OCI registries.
type OCISource struct {
	*types.ZarfPackageOptions
	*ocizarf.ZarfOrasRemote
}

// LoadPackage loads a package from an OCI registry.
func (s *OCISource) LoadPackage(dst *layout.PackagePaths, unarchiveAll bool) (err error) {
	var pkg types.ZarfPackage
	layersToPull := []ocispec.Descriptor{}

	message.Debugf("Loading package from %q", s.PackageSource)

	optionalComponents := helpers.StringToSlice(s.OptionalComponents)

	// pull only needed layers if --confirm is set
	if config.CommonOptions.Confirm {

		layersToPull, err = ocizarf.LayersFromRequestedComponents(s.OrasRemote, optionalComponents)
		if err != nil {
			return fmt.Errorf("unable to get published component image layers: %s", err.Error())
		}
	}

	isPartial := true
	root, err := s.FetchRoot()
	if err != nil {
		return err
	}
	if len(root.Layers) == len(layersToPull) {
		isPartial = false
	}

	layersFetched, err := s.PullPackage(dst.Base, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return fmt.Errorf("unable to pull the package: %w", err)
	}
	dst.SetFromLayers(layersFetched)

	if err := utils.ReadYaml(dst.ZarfYAML, &pkg); err != nil {
		return err
	}

	if err := dst.MigrateLegacy(); err != nil {
		return err
	}

	if !dst.IsLegacyLayout() {
		spinner := message.NewProgressSpinner("Validating pulled layer checksums")
		defer spinner.Stop()

		if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
			return err
		}

		spinner.Success()

		if err := ValidatePackageSignature(dst, s.PublicKeyPath); err != nil {
			return err
		}
	}

	if unarchiveAll {
		for _, component := range pkg.Components {
			if err := dst.Components.Unarchive(component); err != nil {
				if layout.IsNotLoaded(err) {
					_, err := dst.Components.Create(component)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}

		if dst.SBOMs.Path != "" {
			if err := dst.SBOMs.Unarchive(); err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadPackageMetadata loads a package's metadata from an OCI registry.
func (s *OCISource) LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (err error) {
	var pkg types.ZarfPackage

	toPull := ocizarf.PackageAlwaysPull
	if wantSBOM {
		toPull = append(toPull, layout.SBOMTar)
	}

	layersFetched, err := s.PullPackagePaths(toPull, dst.Base)
	if err != nil {
		return err
	}
	dst.SetFromLayers(layersFetched)

	if err := utils.ReadYaml(dst.ZarfYAML, &pkg); err != nil {
		return err
	}

	if err := dst.MigrateLegacy(); err != nil {
		return err
	}

	if !dst.IsLegacyLayout() {
		if wantSBOM {
			spinner := message.NewProgressSpinner("Validating SBOM checksums")
			defer spinner.Stop()

			if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, true); err != nil {
				return err
			}

			spinner.Success()
		}

		if err := ValidatePackageSignature(dst, s.PublicKeyPath); err != nil {
			if errors.Is(err, ErrPkgSigButNoKey) && skipValidation {
				message.Warn("The package was signed but no public key was provided, skipping signature validation")
			} else {
				return err
			}
		}
	}

	// unpack sboms.tar
	if wantSBOM {
		if err := dst.SBOMs.Unarchive(); err != nil {
			return err
		}
	}

	return nil
}

// Collect pulls a package from an OCI registry and writes it to a tarball.
func (s *OCISource) Collect(dir string) (string, error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	fetched, err := s.PullPackage(tmp, config.CommonOptions.OCIConcurrency)
	if err != nil {
		return "", err
	}

	loaded := layout.New(tmp)
	loaded.SetFromLayers(fetched)

	var pkg types.ZarfPackage

	if err := utils.ReadYaml(loaded.ZarfYAML, &pkg); err != nil {
		return "", err
	}

	spinner := message.NewProgressSpinner("Validating full package checksums")
	defer spinner.Stop()

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, false); err != nil {
		return "", err
	}

	spinner.Success()

	// TODO (@Noxsios) remove the suffix check at v1.0.0
	isSkeleton := pkg.Build.Architecture == "skeleton" || strings.HasSuffix(s.Repo().Reference.Reference, oci.SkeletonArch)
	name := NameFromMetadata(&pkg, isSkeleton)

	dstTarball := filepath.Join(dir, name)

	// honor uncompressed flag
	if pkg.Metadata.Uncompressed {
		dstTarball = dstTarball + ".tar"
	} else {
		dstTarball = dstTarball + ".tar.zst"
	}

	allTheLayers, err := filepath.Glob(filepath.Join(tmp, "*"))
	if err != nil {
		return "", err
	}

	_ = os.Remove(dstTarball)

	return dstTarball, archiver.Archive(allTheLayers, dstTarball)
}
