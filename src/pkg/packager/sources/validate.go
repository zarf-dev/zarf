// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

var (
	// ErrPkgKeyButNoSig is returned when a key was provided but the package is not signed
	ErrPkgKeyButNoSig = errors.New("a key was provided but the package is not signed - the package may be corrupted or the --key flag was erroneously specified")
	// ErrPkgSigButNoKey is returned when a package is signed but no key was provided
	ErrPkgSigButNoKey = errors.New("package is signed but no key was provided - add a key with the --key flag or use the --insecure flag and run the command again")
)

// ValidatePackageSignature validates the signature of a package
func ValidatePackageSignature(paths *layout.PackagePaths, publicKeyPath string) error {
	// If the insecure flag was provided ignore the signature validation
	if config.CommonOptions.Insecure {
		return nil
	}

	if publicKeyPath != "" {
		message.Debugf("Using public key %q for signature validation", publicKeyPath)
	}

	// Handle situations where there is no signature within the package
	sigExist := paths.Signature != ""
	if !sigExist && publicKeyPath == "" {
		// Nobody was expecting a signature, so we can just return
		return nil
	} else if sigExist && publicKeyPath == "" {
		// The package is signed but no key was provided
		return ErrPkgSigButNoKey
	} else if !sigExist && publicKeyPath != "" {
		// A key was provided but there is no signature
		return ErrPkgKeyButNoSig
	}

	// Validate the signature with the key we were provided
	if err := utils.CosignVerifyBlob(paths.ZarfYAML, paths.Signature, publicKeyPath); err != nil {
		return fmt.Errorf("package signature did not match the provided key: %w", err)
	}

	return nil
}

// ValidatePackageIntegrity validates the integrity of a package by comparing checksums
func ValidatePackageIntegrity(loaded *layout.PackagePaths, aggregateChecksum string, isPartial bool) error {
	// ensure checksums.txt and zarf.yaml were loaded
	if helpers.InvalidPath(loaded.Checksums) {
		return fmt.Errorf("unable to validate checksums, %s was not loaded", layout.Checksums)
	}
	if helpers.InvalidPath(loaded.ZarfYAML) {
		return fmt.Errorf("unable to validate checksums, %s was not loaded", layout.ZarfYAML)
	}

	checksumPath := loaded.Checksums
	if err := helpers.SHAsMatch(checksumPath, aggregateChecksum); err != nil {
		return err
	}

	checkedMap, err := pathCheckMap(loaded.Base)
	if err != nil {
		return err
	}

	checkedMap[loaded.ZarfYAML] = true
	checkedMap[loaded.Checksums] = true
	checkedMap[loaded.Signature] = true

	err = lineByLine(checksumPath, func(line string) error {
		// If the line is empty (i.e. there is no checksum) simply skip it - this can result from a package with no images/components
		if line == "" {
			return nil
		}

		split := strings.Split(line, " ")
		// If the line is not splitable into two pieces the file is likely corrupted
		if len(split) != 2 {
			return fmt.Errorf("invalid checksum line: %s", line)
		}

		sha := split[0]
		rel := split[1]

		if sha == "" || rel == "" {
			return fmt.Errorf("invalid checksum line: %s", line)
		}
		path := filepath.Join(loaded.Base, rel)

		if helpers.InvalidPath(path) {
			if !isPartial && !checkedMap[path] {
				return fmt.Errorf("unable to validate checksums - missing file: %s", rel)
			} else if isPartial {
				wasLoaded := false
				for rel := range loaded.Files() {
					if path == rel {
						wasLoaded = true
					}
				}
				if wasLoaded {
					return fmt.Errorf("unable to validate partial checksums - missing file: %s", rel)
				}
			}
			// it's okay if we're doing a partial check and the file isn't there as long as the path wasn't loaded
			return nil
		}

		if err := helpers.SHAsMatch(path, sha); err != nil {
			return err
		}

		checkedMap[path] = true

		return nil
	})
	if err != nil {
		return err
	}

	// Make sure we've checked all the files we loaded
	for _, path := range loaded.Files() {
		if !checkedMap[path] {
			return fmt.Errorf("unable to validate loaded checksums, %s did not get checked", path)
		}
	}

	// Check that all of the files in the loaded directory were checked (i.e. no files were weren't expecting got added)
	for path, checked := range checkedMap {
		if !checked {
			return fmt.Errorf("unable to validate checksums, %s did not get checked", path)
		}
	}

	return nil
}

// pathCheckMap returns a map of all the files in a directory and a boolean to use for checking status.
func pathCheckMap(dir string) (map[string]bool, error) {
	filepathMap := make(map[string]bool)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		filepathMap[path] = false
		return err
	})
	return filepathMap, err
}

// lineByLine reads a file line by line and calls a callback function for each line.
func lineByLine(path string, cb func(line string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read line by line
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		err := cb(scanner.Text())
		if err != nil {
			return err
		}
	}
	return nil
}
