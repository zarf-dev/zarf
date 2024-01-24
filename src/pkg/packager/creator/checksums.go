// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// generateChecksums walks through all of the files starting at the base path and generates a checksum file.
// Each file within the basePath represents a layer within the Zarf package.
// generateChecksums returns a SHA256 checksum of the checksums.txt file.
func generateChecksums(dst *layout.PackagePaths) (string, error) {
	// Loop over the "loaded" files
	var checksumsData = []string{}
	for rel, abs := range dst.Files() {
		if rel == layout.ZarfYAML || rel == layout.Checksums {
			continue
		}

		sum, err := utils.GetSHA256OfFile(abs)
		if err != nil {
			return "", err
		}
		checksumsData = append(checksumsData, fmt.Sprintf("%s %s", sum, rel))
	}
	slices.Sort(checksumsData)

	// Create the checksums file
	checksumsFilePath := dst.Checksums
	if err := utils.WriteFile(checksumsFilePath, []byte(strings.Join(checksumsData, "\n")+"\n")); err != nil {
		return "", err
	}

	// Calculate the checksum of the checksum file
	return utils.GetSHA256OfFile(checksumsFilePath)
}
