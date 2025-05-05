// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// verify that OCISource implements PackageSource
	_ PackageSource = (*OCISource)(nil)
)

// OCISource is a package source for OCI registries.
type OCISource struct {
	*types.ZarfPackageOptions
	*zoci.Remote
}

// LoadPackage loads a package from an OCI registry.
func (s *OCISource) LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	pkg, err = s.FetchZarfYAML(ctx)
	if err != nil {
		return pkg, nil, err
	}

	isSkeleton := pkg.Build.Architecture == zoci.SkeletonArch || strings.HasSuffix(s.Repo().Reference.Reference, zoci.SkeletonArch)

	pkg.Components, err = filter.Apply(pkg)
	if err != nil {
		return pkg, nil, err
	}

	layersToPull, err := s.AssembleLayers(ctx, pkg.Components, isSkeleton, "")
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to get published component image layers: %s", err.Error())
	}

	isPartial := true
	root, err := s.FetchRoot(ctx)
	if err != nil {
		return pkg, nil, err
	}
	if len(root.Layers) == len(layersToPull) {
		isPartial = false
	}

	layersFetched, err := s.PullPackage(ctx, dst.Base, config.CommonOptions.OCIConcurrency, layersToPull...)
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to pull the package: %w", err)
	}
	dst.SetFromLayers(ctx, layersFetched)

	if err := dst.MigrateLegacy(ctx); err != nil {
		return pkg, nil, err
	}

	if !dst.IsLegacyLayout() {
		if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, isPartial); err != nil {
			return pkg, nil, err
		}

		if !s.SkipSignatureValidation {
			if err := ValidatePackageSignature(ctx, dst, s.PublicKeyPath); err != nil {
				return pkg, nil, err
			}
		}
	}

	if unarchiveAll {
		for _, component := range pkg.Components {
			if err := dst.Components.Unarchive(ctx, component); err != nil {
				if errors.Is(err, layout.ErrNotLoaded) {
					_, err := dst.Components.Create(component)
					if err != nil {
						return pkg, nil, err
					}
				} else {
					return pkg, nil, err
				}
			}
		}

		if dst.SBOMs.Path != "" {
			if err := dst.SBOMs.Unarchive(); err != nil {
				return pkg, nil, err
			}
		}
	}

	return pkg, warnings, nil
}

// LoadPackageMetadata loads a package's metadata from an OCI registry.
func (s *OCISource) LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	toPull := zoci.PackageAlwaysPull
	if wantSBOM {
		toPull = append(toPull, layout.SBOMTar)
	}
	layersFetched, err := s.PullPaths(ctx, dst.Base, toPull)
	if err != nil {
		return pkg, nil, err
	}
	dst.SetFromLayers(ctx, layersFetched)

	pkg, warnings, err = dst.ReadZarfYAML()
	if err != nil {
		return pkg, nil, err
	}

	if err := dst.MigrateLegacy(ctx); err != nil {
		return pkg, nil, err
	}

	if !dst.IsLegacyLayout() {
		if wantSBOM {
			if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, true); err != nil {
				return pkg, nil, err
			}
		}

		if !s.SkipSignatureValidation {
			if err := ValidatePackageSignature(ctx, dst, s.PublicKeyPath); err != nil {
				if errors.Is(err, ErrPkgSigButNoKey) && skipValidation {
					logger.From(ctx).Warn("the package was signed but no public key was provided, skipping signature validation")
				} else {
					return pkg, nil, err
				}
			}
		}
	}

	// unpack sboms.tar
	if wantSBOM {
		if err := dst.SBOMs.Unarchive(); err != nil {
			return pkg, nil, err
		}
	}

	return pkg, warnings, nil
}

// Collect pulls a package from an OCI registry and writes it to a tarball.
func (s *OCISource) Collect(ctx context.Context, dir string) (string, error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	fetched, err := s.PullPackage(ctx, tmp, config.CommonOptions.OCIConcurrency)
	if err != nil {
		return "", err
	}

	loaded := layout.New(tmp)
	loaded.SetFromLayers(ctx, fetched)

	var pkg v1alpha1.ZarfPackage

	if err := utils.ReadYaml(loaded.ZarfYAML, &pkg); err != nil {
		return "", err
	}

	logger.From(ctx).Debug("validating full package checksums")
	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, false); err != nil {
		return "", err
	}

	// TODO (@Noxsios) remove the suffix check at v1.0.0
	isSkeleton := pkg.Build.Architecture == zoci.SkeletonArch || strings.HasSuffix(s.Repo().Reference.Reference, zoci.SkeletonArch)
	name := fmt.Sprintf("%s%s", NameFromMetadata(&pkg, isSkeleton), PkgSuffix(pkg.Metadata.Uncompressed))

	dstTarball := filepath.Join(dir, name)

	allTheLayers, err := filepath.Glob(filepath.Join(tmp, "*"))
	if err != nil {
		return "", err
	}

	_ = os.Remove(dstTarball)

	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	return dstTarball, archiver.Archive(allTheLayers, dstTarball)
}
