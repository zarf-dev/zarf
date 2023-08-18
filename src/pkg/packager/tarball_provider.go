// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

type tarballProvider struct {
	src string
	dst types.PackagePathsMap
	signatureValidator
}

func (tp *tarballProvider) LoadPackage(optionalComponents []string) (pkg *types.ZarfPackage, err error) {
	if len(optionalComponents) > 0 {
		// TODO: implement
		return nil, nil
	}

	if err := archiver.Unarchive(tp.src, tp.dst.Base()); err != nil {
		return nil, err
	}

	return pkg, utils.ReadYaml(tp.dst[types.ZarfYAML], &pkg)
}

func (tp *tarballProvider) LoadPackageMetadata(wantSBOM bool) (pkg *types.ZarfPackage, err error) {
	pathsToExtract := tp.dst.MetadataPaths()
	if wantSBOM {
		pathsToExtract[types.ZarfSBOMTar] = tp.dst[types.ZarfSBOMTar]
	}
	for pathInArchive := range pathsToExtract {
		if err := archiver.Extract(tp.src, pathInArchive, tp.dst.Base()); err != nil {
			return nil, err
		}
	}

	return pkg, utils.ReadYaml(tp.dst[types.ZarfYAML], &pkg)
}

// func (p *Packager) handleIfPartialPkg() error {
// 	message.Debugf("Checking for partial package: %s", p.cfg.PkgOpts.PackagePath)

// 	// If packagePath has partial in the name, we need to combine the partials into a single package
// 	if !strings.Contains(p.cfg.PkgOpts.PackagePath, ".part000") {
// 		message.Debug("No partial package detected")
// 		return nil
// 	}

// 	message.Debug("Partial package detected")

// 	// Replace part 000 with *
// 	pattern := strings.Replace(p.cfg.PkgOpts.PackagePath, ".part000", ".part*", 1)
// 	fileList, err := filepath.Glob(pattern)
// 	if err != nil {
// 		return fmt.Errorf("unable to find partial package files: %s", err)
// 	}

// 	// Ensure the files are in order so they are appended in the correct order
// 	sort.Strings(fileList)

// 	// Create the new package
// 	destination := strings.Replace(p.cfg.PkgOpts.PackagePath, ".part000", "", 1)
// 	pkgFile, err := os.Create(destination)
// 	if err != nil {
// 		return fmt.Errorf("unable to create new package file: %s", err)
// 	}
// 	defer pkgFile.Close()

// 	// Remove the new package if there is an error
// 	defer func() {
// 		// If there is an error, remove the new package
// 		if p.cfg.PkgOpts.PackagePath != destination {
// 			os.Remove(destination)
// 		}
// 	}()

// 	var pgkData types.ZarfPartialPackageData

// 	// Loop through the partial packages and append them to the new package
// 	for idx, file := range fileList {
// 		// The first file contains metadata about the package
// 		if idx == 0 {
// 			var bytes []byte

// 			if bytes, err = os.ReadFile(file); err != nil {
// 				return fmt.Errorf("unable to read file %s: %w", file, err)
// 			}

// 			if err := json.Unmarshal(bytes, &pgkData); err != nil {
// 				return fmt.Errorf("unable to unmarshal file %s: %w", file, err)
// 			}

// 			count := len(fileList) - 1
// 			if count != pgkData.Count {
// 				return fmt.Errorf("package is missing parts, expected %d, found %d", pgkData.Count, count)
// 			}

// 			continue
// 		}

// 		// Open the file
// 		f, err := os.Open(file)
// 		if err != nil {
// 			return fmt.Errorf("unable to open file %s: %w", file, err)
// 		}
// 		defer f.Close()

// 		// Add the file contents to the package
// 		if _, err = io.Copy(pkgFile, f); err != nil {
// 			return fmt.Errorf("unable to copy file %s: %w", file, err)
// 		}
// 	}

// 	var shasum string
// 	if shasum, err = utils.GetCryptoHashFromFile(destination, crypto.SHA256); err != nil {
// 		return fmt.Errorf("unable to get sha256sum of package: %w", err)
// 	}

// 	if shasum != pgkData.Sha256Sum {
// 		return fmt.Errorf("package sha256sum does not match, expected %s, found %s", pgkData.Sha256Sum, shasum)
// 	}

// 	// Remove the partial packages to reduce disk space before extracting
// 	for _, file := range fileList {
// 		_ = os.Remove(file)
// 	}

// 	// Success, update the package path
// 	p.cfg.PkgOpts.PackagePath = destination
// 	return nil
// }
