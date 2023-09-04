// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

// TarballSource is a package source for tarballs.
type TarballSource struct {
	DestinationDir string
	*types.ZarfPackageOptions
}

// LoadPackage loads a package from a tarball.
func (s *TarballSource) LoadPackage(_ []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = s.DestinationDir

	message.Debugf("Loading package from %q", s.PackageSource)
	message.Debugf("Loaded package base directory: %q", s.DestinationDir)

	err = archiver.Walk(s.PackageSource, func(f archiver.File) error {
		if f.IsDir() {
			return nil
		}
		header, ok := f.Header.(*tar.Header)
		if !ok {
			return fmt.Errorf("expected header to be *tar.Header but was %T", f.Header)
		}
		fullPath := header.Name

		dir := filepath.Dir(fullPath)
		if dir != "." {
			if err := os.MkdirAll(filepath.Join(s.DestinationDir, dir), 0755); err != nil {
				return err
			}
		}

		dstPath := filepath.Join(s.DestinationDir, fullPath)
		dst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, f)
		if err != nil {
			return err
		}

		loaded[fullPath] = dstPath
		message.Debugf("Loaded %q --> %q", fullPath, dstPath)
		return nil
	})
	if err != nil {
		return pkg, nil, err
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, false); err != nil {
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

// LoadPackageMetadata loads a package's metadata from a tarball.
func (s *TarballSource) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = s.DestinationDir

	for pathInArchive := range loaded.MetadataPaths() {
		if err := archiver.Extract(s.PackageSource, pathInArchive, s.DestinationDir); err != nil {
			return pkg, nil, err
		}
		pathOnDisk := filepath.Join(s.DestinationDir, pathInArchive)
		if !utils.InvalidPath(pathOnDisk) {
			loaded[pathInArchive] = pathOnDisk
		}
	}
	if wantSBOM {
		if err := archiver.Extract(s.PackageSource, types.SBOMTar, s.DestinationDir); err != nil {
			return pkg, nil, err
		}
		pathOnDisk := filepath.Join(s.DestinationDir, types.SBOMTar)
		if !utils.InvalidPath(pathOnDisk) {
			loaded[types.SBOMTar] = pathOnDisk
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
	if _, ok := loaded[types.SBOMTar]; ok {
		loaded[types.SBOMDir] = filepath.Join(s.DestinationDir, types.SBOMDir)
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
			return pkg, nil, err
		}
	} else if wantSBOM {
		return pkg, nil, fmt.Errorf("package does not contain SBOMs")
	}

	return pkg, loaded, nil
}

// Collect for the TarballSource is essentially an `mv`
func (s *TarballSource) Collect(destinationTarball string) error {
	return os.Rename(s.PackageSource, destinationTarball)
}

// PartialTarballSource is a package source for partial tarballs.
type PartialTarballSource struct {
	DestinationDir string
	*types.ZarfPackageOptions
}

// Collect turns a partial tarball into a full tarball.
func (s *PartialTarballSource) Collect(dstTarball string) error {
	pattern := strings.Replace(s.PackageSource, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find partial package files: %s", err)
	}

	// Ensure the files are in order so they are appended in the correct order
	sort.Strings(fileList)

	// Create the new package
	pkgFile, err := os.Create(dstTarball)
	if err != nil {
		return fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	var pkgData types.ZarfPartialPackageData
	for idx, file := range fileList {
		// The first file contains metadata about the package
		if idx == 0 {
			var bytes []byte

			if bytes, err = os.ReadFile(file); err != nil {
				return fmt.Errorf("unable to read file %s: %w", file, err)
			}

			if err := json.Unmarshal(bytes, &pkgData); err != nil {
				return fmt.Errorf("unable to unmarshal file %s: %w", file, err)
			}

			count := len(fileList) - 1
			if count != pkgData.Count {
				return fmt.Errorf("package is missing parts, expected %d, found %d", pkgData.Count, count)
			}

			if len(s.Shasum) > 0 && pkgData.Sha256Sum != s.Shasum {
				return fmt.Errorf("mismatch in CLI options and package metadata, expected %s, found %s", s.Shasum, pkgData.Sha256Sum)
			}

			continue
		}

		// Open the file
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("unable to open file %s: %w", file, err)
		}
		defer f.Close()

		// Add the file contents to the package
		if _, err = io.Copy(pkgFile, f); err != nil {
			return fmt.Errorf("unable to copy file %s: %w", file, err)
		}
	}

	var shasum string
	if shasum, err = utils.GetSHA256OfFile(dstTarball); err != nil {
		return fmt.Errorf("unable to get sha256sum of package: %w", err)
	}

	if shasum != pkgData.Sha256Sum {
		return fmt.Errorf("package sha256sum does not match, expected %s, found %s", pkgData.Sha256Sum, shasum)
	}

	// Remove the partial packages to reduce disk space before extracting
	for _, file := range fileList {
		_ = os.Remove(file)
	}

	message.Infof("Reassembled package: %q", filepath.Base(dstTarball))

	return nil
}

// LoadPackage loads a package from a partial tarball.
func (s *PartialTarballSource) LoadPackage(optionalComponents []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	dstTarball := strings.Replace(s.PackageSource, ".part000", "", 1)

	if err := s.Collect(dstTarball); err != nil {
		_ = os.Remove(dstTarball)
		return pkg, nil, err
	}

	// Update the package source to the reassembled tarball
	s.PackageSource = dstTarball

	tp := &TarballSource{
		s.DestinationDir,
		s.ZarfPackageOptions,
	}
	return tp.LoadPackage(optionalComponents)
}

// LoadPackageMetadata loads a package's metadata from a partial tarball.
func (s *PartialTarballSource) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	dstTarball := strings.Replace(s.PackageSource, ".part000", "", 1)

	if err := s.Collect(dstTarball); err != nil {
		_ = os.Remove(dstTarball)
		return pkg, nil, err
	}

	// Update the package source to the reassembled tarball
	s.PackageSource = dstTarball

	tp := &TarballSource{
		s.DestinationDir,
		s.ZarfPackageOptions,
	}
	return tp.LoadPackageMetadata(wantSBOM)
}
