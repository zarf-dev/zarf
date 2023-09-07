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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// ErrPkgKeyButNoSig is returned when a key was provided but the package is not signed
	ErrPkgKeyButNoSig = errors.New("a key was provided but the package is not signed - remove the --key flag and run the command again")
	// ErrPkgSigButNoKey is returned when a package is signed but no key was provided
	ErrPkgSigButNoKey = errors.New("package is signed but no key was provided - add a key with the --key flag or use the --insecure flag and run the command again")
)

// ValidatePackageSignature validates the signature of a package
func ValidatePackageSignature(paths types.PackagePathsMap, publicKeyPath string) error {
	// If the insecure flag was provided ignore the signature validation
	if config.CommonOptions.Insecure {
		return nil
	}

	if publicKeyPath != "" {
		message.Debugf("Using public key %q for signature validation", publicKeyPath)
	}

	// Handle situations where there is no signature within the package
	sigExist := !utils.InvalidPath(paths[types.PackageSignature])
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
	if err := utils.CosignVerifyBlob(paths[types.ZarfYAML], paths[types.PackageSignature], publicKeyPath); err != nil {
		return fmt.Errorf("package signature did not match the provided key: %w", err)
	}

	return nil
}

// ValidatePackageIntegrity validates the integrity of a package by comparing checksums
func ValidatePackageIntegrity(loaded types.PackagePathsMap, aggregateChecksum string, isPartial bool) error {
	spinner := message.NewProgressSpinner("Validating package checksums")
	defer spinner.Stop()

	// ensure checksums.txt and zarf.yaml were loaded
	if !loaded.KeyExists(types.PackageChecksums) {
		// TODO: right now older packages (the SGET one in CI) do not have checksums.txt
		// disabling this check for now, but we should re-enable it once we have a new SGET package
		if aggregateChecksum == "" {
			spinner.Successf("Checksums validated!")
			return nil
		}
		return fmt.Errorf("unable to validate checksums, %s was not loaded", types.PackageChecksums)
	}
	if !loaded.KeyExists(types.ZarfYAML) {
		return fmt.Errorf("unable to validate checksums, %s was not loaded", types.ZarfYAML)
	}

	checksumPath := loaded[types.PackageChecksums]
	if err := utils.SHAsMatch(checksumPath, aggregateChecksum); err != nil {
		return err
	}

	checkedMap, err := pathCheckMap(loaded.Base())
	if err != nil {
		return err
	}

	for _, rel := range loaded.MetadataKeys() {
		checkedMap[filepath.Join(loaded.Base(), rel)] = true
	}

	err = lineByLine(checksumPath, func(line string) error {
		split := strings.Split(line, " ")
		sha := split[0]
		rel := split[1]
		if sha == "" || rel == "" {
			return fmt.Errorf("invalid checksum line: %s", line)
		}
		path := filepath.Join(loaded.Base(), rel)

		status := fmt.Sprintf("Validating checksum of %s", utils.First30last30(rel))
		spinner.Updatef(status)

		if utils.InvalidPath(path) {
			if !isPartial && !checkedMap[path] {
				return fmt.Errorf("unable to validate checksums - missing file: %s", rel)
			} else if loaded.KeyExists(rel) {
				return fmt.Errorf("unable to validate partial checksums - missing file: %s", rel)
			}
			// it's okay if we're doing a partial check and the file isn't there as long as the path isn't in the list of paths to check
			return nil
		}

		actualSHA, err := utils.GetSHA256OfFile(path)
		if err != nil {
			return fmt.Errorf("unable to get checksum of: %s", err.Error())
		}

		if sha != actualSHA {
			return fmt.Errorf("invalid checksum for %s: (expected: %s, received: %s)", path, sha, actualSHA)
		}

		checkedMap[path] = true

		return nil
	})
	if err != nil {
		return err
	}

	// If we're doing a partial check, make sure we've checked all the files we were asked to check
	if isPartial {
		for rel, path := range loaded {
			if rel == types.BaseDir {
				continue
			}
			if !checkedMap[path] {
				return fmt.Errorf("unable to validate partial checksums, %s did not get checked", path)
			}
		}
	}

	for path, checked := range checkedMap {
		if !checked {
			return fmt.Errorf("unable to validate checksums, %s did not get checked", path)
		}
	}

	spinner.Successf("Checksums validated!")

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
		return nil
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
