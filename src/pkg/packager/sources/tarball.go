// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/ocizarf"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

var (
	// veryify that TarballSource implements PackageSource
	_ PackageSource = (*TarballSource)(nil)
)

// TarballSource is a package source for tarballs.
type TarballSource struct {
	*types.ZarfPackageOptions
}

// LoadPackage loads a package from a tarball.
func (s *TarballSource) LoadPackage(dst *layout.PackagePaths, unarchiveAll bool) (err error) {
	var pkg types.ZarfPackage

	spinner := message.NewProgressSpinner("Loading package from %q", s.PackageSource)
	defer spinner.Stop()

	if s.Shasum != "" {
		if err := utils.SHAsMatch(s.PackageSource, s.Shasum); err != nil {
			return err
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
			if err := os.MkdirAll(filepath.Join(dst.Base, dir), 0755); err != nil {
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
		return err
	}

	dst.SetFromPaths(pathsExtracted)

	if err := utils.ReadYaml(dst.ZarfYAML, &pkg); err != nil {
		return err
	}

	if err := dst.MigrateLegacy(); err != nil {
		return err
	}

	if !dst.IsLegacyLayout() {
		spinner := message.NewProgressSpinner("Validating full package checksums")
		defer spinner.Stop()

		if err := ValidatePackageIntegrity(dst, pkg.Metadata.AggregateChecksum, false); err != nil {
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

	spinner.Success()

	return nil
}

// LoadPackageMetadata loads a package's metadata from a tarball.
func (s *TarballSource) LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (err error) {
	var pkg types.ZarfPackage

	if s.Shasum != "" {
		if err := utils.SHAsMatch(s.PackageSource, s.Shasum); err != nil {
			return err
		}
	}

	toExtract := ocizarf.PackageAlwaysPull
	if wantSBOM {
		toExtract = append(toExtract, layout.SBOMTar)
	}
	pathsExtracted := []string{}

	for _, rel := range toExtract {
		if err := archiver.Extract(s.PackageSource, rel, dst.Base); err != nil {
			return err
		}
		// archiver.Extract will not return an error if the file does not exist, so we must manually check
		if !utils.InvalidPath(filepath.Join(dst.Base, rel)) {
			pathsExtracted = append(pathsExtracted, rel)
		}
	}

	dst.SetFromPaths(pathsExtracted)

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

	if wantSBOM {
		if err := dst.SBOMs.Unarchive(); err != nil {
			return err
		}
	}

	return nil
}

// Collect for the TarballSource is essentially an `mv`
func (s *TarballSource) Collect(dir string) (string, error) {
	dst := filepath.Join(dir, filepath.Base(s.PackageSource))
	return dst, os.Rename(s.PackageSource, dst)
}
