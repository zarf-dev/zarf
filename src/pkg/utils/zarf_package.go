// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"
)

// IsInitConfig returns whether the provided Zarf package is an init config.
func IsInitConfig(pkg types.ZarfPackage) bool {
	return pkg.Kind == types.ZarfInitConfig
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName(arch string) string {
	if arch == "" {
		// No package has been loaded yet so lookup GetArch() with no package info
		arch = config.GetArch()
	}
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// GetPackageName returns the formatted name of the package.
func GetPackageName(pkg types.ZarfPackage, diffData types.DifferentialData) string {
	if IsInitConfig(pkg) {
		return GetInitPackageName(pkg.Metadata.Architecture)
	}

	packageName := pkg.Metadata.Name
	suffix := "tar.zst"
	if pkg.Metadata.Uncompressed {
		suffix = "tar"
	}

	packageFileName := fmt.Sprintf("%s%s-%s", config.ZarfPackagePrefix, packageName, pkg.Metadata.Architecture)
	if pkg.Build.Differential {
		packageFileName = fmt.Sprintf("%s-%s-differential-%s", packageFileName, diffData.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		packageFileName = fmt.Sprintf("%s-%s", packageFileName, pkg.Metadata.Version)
	}

	return fmt.Sprintf("%s.%s", packageFileName, suffix)
}

// IsSBOMAble checks if a package has contents that an SBOM can be created on (i.e. images, files, or data injections)
func IsSBOMAble(pkg types.ZarfPackage) bool {
	for _, c := range pkg.Components {
		if len(c.Images) > 0 || len(c.Files) > 0 || len(c.DataInjections) > 0 {
			return true
		}
	}

	return false
}
