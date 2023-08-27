// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

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

	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

// TarballProvider is a package provider for tarballs.
type TarballProvider struct {
	source         string
	destinationDir string
	opts           *types.ZarfPackageOptions
}

// LoadPackage loads a package from a tarball.
func (tp *TarballProvider) LoadPackage(_ []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = tp.destinationDir

	err = archiver.Walk(tp.source, func(f archiver.File) error {
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
			if err := os.MkdirAll(filepath.Join(tp.destinationDir, dir), 0755); err != nil {
				return err
			}
		}

		dstPath := filepath.Join(tp.destinationDir, fullPath)
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
		return nil
	})
	if err != nil {
		return pkg, nil, err
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, false); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, tp.opts.PublicKeyPath); err != nil {
		return pkg, nil, err
	}

	// always create and "load" components dir
	if _, ok := loaded[types.ZarfComponentsDir]; !ok {
		loaded[types.ZarfComponentsDir] = filepath.Join(tp.destinationDir, types.ZarfComponentsDir)
		if err := utils.CreateDirectory(loaded[types.ZarfComponentsDir], 0755); err != nil {
			return pkg, nil, err
		}
	}

	// unpack component tarballs
	for _, component := range pkg.Components {
		tb := filepath.Join(types.ZarfComponentsDir, fmt.Sprintf("%s.tar", component.Name))
		if _, ok := loaded[tb]; ok {
			defer os.Remove(loaded[tb])
			defer delete(loaded, tb)
			if err = archiver.Unarchive(loaded[tb], loaded[types.ZarfComponentsDir]); err != nil {
				return pkg, nil, err
			}
		}

		// also "load" the images dir if any component has images
		if _, ok := loaded[types.ZarfImagesDir]; !ok && len(component.Images) > 0 {
			loaded[types.ZarfImagesDir] = filepath.Join(tp.destinationDir, types.ZarfImagesDir)
			if err := utils.CreateDirectory(loaded[types.ZarfImagesDir], 0755); err != nil {
				return pkg, nil, err
			}
		}
	}

	// unpack sboms.tar
	if _, ok := loaded[types.ZarfSBOMTar]; ok {
		loaded[types.ZarfSBOMDir] = filepath.Join(tp.destinationDir, types.ZarfSBOMDir)
		if err = archiver.Unarchive(loaded[types.ZarfSBOMTar], loaded[types.ZarfSBOMDir]); err != nil {
			return pkg, nil, err
		}
	}

	return pkg, loaded, nil
}

// LoadPackageMetadata loads a package's metadata from a tarball.
func (tp *TarballProvider) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	loaded = make(types.PackagePathsMap)
	loaded[types.BaseDir] = tp.destinationDir

	for pathInArchive := range loaded.MetadataPaths() {
		if err := archiver.Extract(tp.source, pathInArchive, tp.destinationDir); err != nil {
			return pkg, nil, err
		}
		loaded[pathInArchive] = filepath.Join(tp.destinationDir, pathInArchive)
	}
	if wantSBOM {
		if err := archiver.Extract(tp.source, types.ZarfSBOMTar, tp.destinationDir); err != nil {
			return pkg, nil, err
		}
		loaded[types.ZarfSBOMTar] = filepath.Join(tp.destinationDir, types.ZarfSBOMTar)
	}

	if err := utils.ReadYaml(loaded[types.ZarfYAML], &pkg); err != nil {
		return pkg, nil, err
	}

	if err := validate.PackageIntegrity(loaded, pkg.Metadata.AggregateChecksum, true); err != nil {
		return pkg, nil, err
	}

	if err := ValidatePackageSignature(loaded, tp.opts.PublicKeyPath); err != nil {
		if errors.Is(err, ErrPkgSigButNoKey) {
			message.Warn("The package was signed but no public key was provided, skipping signature validation")
		} else {
			return pkg, nil, err
		}
	}

	// unpack sboms.tar
	if _, ok := loaded[types.ZarfSBOMTar]; ok {
		loaded[types.ZarfSBOMDir] = filepath.Join(tp.destinationDir, types.ZarfSBOMDir)
		if err = archiver.Unarchive(loaded[types.ZarfSBOMTar], loaded[types.ZarfSBOMDir]); err != nil {
			return pkg, nil, err
		}
	} else if wantSBOM {
		return pkg, nil, fmt.Errorf("package does not contain SBOMs")
	}

	return pkg, loaded, nil
}

// PartialTarballProvider is a package provider for partial tarballs.
type PartialTarballProvider struct {
	source         string
	outputTarball  string
	destinationDir string
	opts           *types.ZarfPackageOptions
}

// reassembleTarball reassembles the partial tarball into a single tarball.
func (ptp *PartialTarballProvider) reassembleTarball() (err error) {
	pattern := strings.Replace(ptp.source, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find partial package files: %s", err)
	}

	// Ensure the files are in order so they are appended in the correct order
	sort.Strings(fileList)

	// Create the new package
	destination := strings.Replace(ptp.source, ".part000", "", 1)
	pkgFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	// Remove the new package if there is an error
	defer func() {
		// If there is an error, remove the new package
		if ptp.outputTarball != destination {
			os.Remove(destination)
		}
	}()

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

			if len(ptp.opts.Shasum) > 0 && pkgData.Sha256Sum != ptp.opts.Shasum {
				return fmt.Errorf("mismatch in CLI options and package metadata, expected %s, found %s", ptp.opts.Shasum, pkgData.Sha256Sum)
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
	if shasum, err = utils.GetSHA256OfFile(destination); err != nil {
		return fmt.Errorf("unable to get sha256sum of package: %w", err)
	}

	if shasum != pkgData.Sha256Sum {
		return fmt.Errorf("package sha256sum does not match, expected %s, found %s", pkgData.Sha256Sum, shasum)
	}

	// Remove the partial packages to reduce disk space before extracting
	for _, file := range fileList {
		_ = os.Remove(file)
	}

	ptp.outputTarball = destination

	message.Infof("Reassembled package: %s", filepath.Base(ptp.outputTarball))

	return nil
}

// LoadPackage loads a package from a partial tarball.
func (ptp *PartialTarballProvider) LoadPackage(optionalComponents []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := ptp.reassembleTarball(); err != nil {
		return pkg, nil, err
	}

	tp := &TarballProvider{
		source:         ptp.outputTarball,
		destinationDir: ptp.destinationDir,
		opts:           ptp.opts,
	}
	return tp.LoadPackage(optionalComponents)
}

// LoadPackageMetadata loads a package's metadata from a partial tarball.
func (ptp *PartialTarballProvider) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := ptp.reassembleTarball(); err != nil {
		return pkg, nil, err
	}

	tp := &TarballProvider{
		source:         ptp.outputTarball,
		destinationDir: ptp.destinationDir,
		opts:           ptp.opts,
	}
	return tp.LoadPackageMetadata(wantSBOM)
}
