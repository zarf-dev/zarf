// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"os"
	"path/filepath"
)

// OutputSBOMFiles outputs the sbom files into a specified directory.
func OutputSBOMFiles(sourceDir, outputDir, packageName string) (string, error) {
	packagePath := filepath.Join(outputDir, packageName)

	if err := os.RemoveAll(packagePath); err != nil {
		return "", err
	}

	if err := CreateDirectory(packagePath, 0700); err != nil {
		return "", err
	}

	return packagePath, CreatePathAndCopy(sourceDir, packagePath)
}
