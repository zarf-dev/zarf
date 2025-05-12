// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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
	// 1) Mount the archive as a virtual file system.
	fsys, err := archives.FileSystem(ctx, s.PackageSource, nil)
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to open archive %q: %w", s.PackageSource, err)
	}

	// 2) Walk every entry in the archive.
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip directories
		if d.IsDir() {
			return nil
		}
		// ensure parent dirs exist in our temp dir
		dstPath := filepath.Join(dst.Base, path)
		pathsExtracted = append(pathsExtracted, path)
		if err := os.MkdirAll(filepath.Dir(dstPath), helpers.ReadExecuteAllWriteUser); err != nil {
			return err
		}
		// copy file contents
		in, err := fsys.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
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
		l.Info("validating package checksums", "source", s.PackageSource)

		if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, false); err != nil {
			return pkg, nil, err
		}

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

	decompressOpts := archive.DecompressOpts{
		Files: toExtract,
	}
	err = archive.Decompress(ctx, s.PackageSource, dst.Base, decompressOpts)
	if err != nil {
		return pkg, nil, fmt.Errorf("unable to extract archive %q: %w", s.PackageSource, err)
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
