// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package validate provides Zarf package validation functions.
package validate

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

func PackageIntegrity(loaded types.PackagePathsMap, pathsToCheck []string, aggregateChecksum string) error {
	spinner := message.NewProgressSpinner("Validating package checksums")
	defer spinner.Stop()

	checksumPath := loaded[types.ZarfChecksumsTxt]
	actualAggregateChecksum, err := utils.GetSHA256OfFile(checksumPath)
	if err != nil {
		return fmt.Errorf("unable to get checksum of: %s", err.Error())
	}
	if actualAggregateChecksum != aggregateChecksum {
		return fmt.Errorf("invalid aggregate checksum: (expected: %s, received: %s)", aggregateChecksum, actualAggregateChecksum)
	}

	isPartial := false
	if len(pathsToCheck) > 0 {
		pathsToCheck = helpers.Unique(pathsToCheck)
		isPartial = true
		message.Debugf("Validating checksums for a subset of files in the package - %v", pathsToCheck)
		for idx, path := range pathsToCheck {
			pathsToCheck[idx] = filepath.Join(loaded.Base(), path)
		}
	}

	checkedMap, err := pathCheckMap(loaded.Base())
	if err != nil {
		return err
	}

	for _, abs := range loaded.MetadataPaths() {
		checkedMap[abs] = true
	}

	err = lineByLine(checksumPath, func(line string) error {
		split := strings.Split(line, " ")
		sha := split[0]
		rel := split[1]
		if sha == "" || rel == "" {
			return fmt.Errorf("invalid checksum line: %s", line)
		}
		path := filepath.Join(loaded.Base(), rel)

		status := fmt.Sprintf("Validating checksum of %s", rel)
		spinner.Updatef(message.Truncate(status, message.TermWidth, false))

		if utils.InvalidPath(path) {
			if !isPartial && !checkedMap[path] {
				return fmt.Errorf("unable to validate checksums - missing file: %s", rel)
			} else if helpers.SliceContains(pathsToCheck, path) {
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
		for _, path := range pathsToCheck {
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
