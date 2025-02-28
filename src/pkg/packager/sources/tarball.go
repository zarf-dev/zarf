// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// verify that TarballSource implements PackageSource
	_ PackageSource = (*TarballSource)(nil)
)

// TarballSource is a package source for tarballs.
type TarballSource struct {
	*types.ZarfPackageOptions
}

// LoadPackage loads a package from a tarball.
func (s *TarballSource) LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	l := logger.From(ctx)
	start := time.Now()
	l.Info("loading package", "source", s.PackageSource)

	if s.Shasum != "" {
		if err := helpers.SHAsMatch(s.PackageSource, s.Shasum); err != nil {
			return pkg, nil, err
		}
	}

	pathsExtracted := []string{}

	err = archiver.Walk(s.PackageSource, func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		header, ok := f.Header.(*tar.Header)
		if !ok {
			return fmt.Errorf("expected header to be *tar.Header but was %T", f.Header)
		}
		path := header.Name

		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(filepath.Join(dst.Base, dir), helpers.ReadExecuteAllWriteUser); err != nil {
				return err
			}
		}

		dstPath := filepath.Join(dst.Base, path)
		pathsExtracted = append(pathsExtracted, path)
		dst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, f)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return pkg, nil, err
	}

	dst.SetFromPaths(ctx, pathsExtracted)

	pkg, warnings, err = dst.ReadZarfYAML()
	if err != nil {
		return pkg, nil, err
	}
	pkg.Components, err = filter.Apply(pkg)
	if err != nil {
		return pkg, nil, err
	}

	if err := dst.MigrateLegacy(ctx); err != nil {
		return pkg, nil, err
	}

	if !dst.IsLegacyLayout() {
		spinner := message.NewProgressSpinner("Validating full package checksums")
		defer spinner.Stop()
		l.Info("validating package checksums", "source", s.PackageSource)

		if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, false); err != nil {
			return pkg, nil, err
		}

		spinner.Success()
		l.Debug("done validating package checksums", "source", s.PackageSource)

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

	l.Debug("done loading package", "source", s.PackageSource, "duration", time.Since(start))

	return pkg, warnings, nil
}

// LoadPackageMetadata loads a package's metadata from a tarball.
func (s *TarballSource) LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error) {
	if s.Shasum != "" {
		if err := helpers.SHAsMatch(s.PackageSource, s.Shasum); err != nil {
			return pkg, nil, err
		}
	}

	toExtract := zoci.PackageAlwaysPull
	if wantSBOM {
		toExtract = append(toExtract, layout.SBOMTar)
	}
	pathsExtracted := []string{}

	for _, rel := range toExtract {
		if err := archiver.Extract(s.PackageSource, rel, dst.Base); err != nil {
			return pkg, nil, err
		}
		// archiver.Extract will not return an error if the file does not exist, so we must manually check
		if !helpers.InvalidPath(filepath.Join(dst.Base, rel)) {
			pathsExtracted = append(pathsExtracted, rel)
		}
	}

	dst.SetFromPaths(ctx, pathsExtracted)

	pkg, warnings, err = dst.ReadZarfYAML()
	if err != nil {
		return pkg, nil, err
	}

	if err := dst.MigrateLegacy(ctx); err != nil {
		return pkg, nil, err
	}

	if !dst.IsLegacyLayout() {
		if wantSBOM {
			spinner := message.NewProgressSpinner("Validating SBOM checksums")
			defer spinner.Stop()

			if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, true); err != nil {
				return pkg, nil, err
			}

			spinner.Success()
		}

		if !s.SkipSignatureValidation {
			if err := ValidatePackageSignature(ctx, dst, s.PublicKeyPath); err != nil {
				if errors.Is(err, ErrPkgSigButNoKey) && skipValidation {
					message.Warn("The package was signed but no public key was provided, skipping signature validation")
					logger.From(ctx).Warn("the package was signed but no public key was provided, skipping signature validation")
				} else {
					return pkg, nil, err
				}
			}
		}
	}

	if wantSBOM {
		if err := dst.SBOMs.Unarchive(); err != nil {
			return pkg, nil, err
		}
	}

	return pkg, warnings, nil
}

// Collect for the TarballSource is essentially an `mv`
func (s *TarballSource) Collect(_ context.Context, dir string) (string, error) {
	dst := filepath.Join(dir, filepath.Base(s.PackageSource))
	err := os.Rename(s.PackageSource, dst)
	linkErr := &os.LinkError{}
	isLinkErr := errors.As(err, &linkErr)
	if err != nil && !isLinkErr {
		return "", err
	}
	if err == nil {
		return dst, nil
	}

	// Copy file if rename is not possible due to existing on different partitions.
	srcFile, err := os.Open(linkErr.Old)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(linkErr.New)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return "", err
	}
	err = os.Remove(linkErr.Old)
	if err != nil {
		return "", err
	}
	return dst, nil
}
