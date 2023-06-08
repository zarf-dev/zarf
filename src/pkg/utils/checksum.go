// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// ValidatePackageChecksums validates the checksums of a Zarf package.
func ValidatePackageChecksums(baseDir string, checksumPath string, aggregateChecksum string, pathsToCheck []string) error {
	spinner := message.NewProgressSpinner("Validating package checksums")
	defer spinner.Stop()

	// Run pre-checks to make sure we have what we need to validate the checksums
	if InvalidPath(baseDir) {
		return fmt.Errorf("invalid base directory: %s", baseDir)
	}
	if InvalidPath(checksumPath) {
		return fmt.Errorf("invalid checksum file path: %s", checksumPath)
	}
	if aggregateChecksum == "" {
		return fmt.Errorf("unable to validate checksums because of missing metadata checksum signature")
	}
	if len(aggregateChecksum) != 64 {
		return fmt.Errorf("invalid aggregate checksum: %s", aggregateChecksum)
	}
	isPartial := false
	if len(pathsToCheck) > 0 {
		isPartial = true
	}

	pathCheckMap, err := PathCheckMap(baseDir)
	if err != nil {
		return err
	}

	actualAggregateChecksum, err := GetSHA256OfFile(checksumPath)
	if err != nil {
		return fmt.Errorf("unable to get checksum of: %s", err.Error())
	}
	if actualAggregateChecksum != aggregateChecksum {
		return fmt.Errorf("invalid aggregate checksum: (expected: %s, received: %s)", aggregateChecksum, actualAggregateChecksum)
	}

	// this checksum will not match as the checksums.txt's checksum is added to zarf.yaml after the checksums.txt is generated
	pathCheckMap[config.ZarfYAML] = true
	pathCheckMap[config.ZarfYAMLSignature] = true

	err = LineByLine(checksumPath, func(line string) error {
		split := strings.Split(line, " ")
		sha := split[0]
		rel := split[1]
		if sha == "" || rel == "" {
			return fmt.Errorf("invalid checksum line: %s", line)
		}
		path := filepath.Join(baseDir, rel)

		status := fmt.Sprintf("Validating checksum of %s", rel)
		if len(status) > message.TermWidth {
			max := message.TermWidth - 3
			status = fmt.Sprintf("%s...", status[:max])
		}
		spinner.Updatef(status)

		if InvalidPath(path) {
			if isPartial && SliceContains(pathsToCheck, rel) {
				return fmt.Errorf("unable to validate checksums because of missing file: %s", rel)
			}
			message.Debugf("Skipping checksum validation for missing file: %s", rel)
			return nil
		}

		actualSHA, err := GetSHA256OfFile(path)
		if err != nil {
			return fmt.Errorf("unable to get checksum of: %s", err.Error())
		}

		if sha != actualSHA {
			return fmt.Errorf("invalid checksum for %s: (expected: %s, received: %s)", path, sha, actualSHA)
		}

		pathCheckMap[path] = true

		return nil
	})
	if err != nil {
		return err
	}

	// If we're doing a partial check, make sure we've checked all the files we were asked to check
	if isPartial {
		for _, path := range pathsToCheck {
			if !pathCheckMap[path] {
				return fmt.Errorf("unable to validate checksums because of missing file: %s", path)
			}
		}
	} else {
		// Otherwise, make sure we've checked all the files in the package
		for path, checked := range pathCheckMap {
			if !checked {
				return fmt.Errorf("unable to validate checksums because of missing file: %s", path)
			}
		}
	}

	spinner.Successf("All of the checksums matched!")
	return nil
}

// PathCheckMap returns a map of all the files in a directory and a boolean to use for checking status.
func PathCheckMap(dir string) (map[string]bool, error) {
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

// LineByLine reads a file line by line and calls a callback function for each line.
func LineByLine(path string, cb func(line string) error) error {
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
