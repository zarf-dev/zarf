// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// GetValidPackageExtensions returns the valid package extensions.
func GetValidPackageExtensions() [2]string {
	return [...]string{".tar.zst", ".tar"}
}

// IsValidFileExtension returns true if the filename has a valid package extension.
func IsValidFileExtension(filename string) bool {
	for _, extension := range GetValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			return true
		}
	}

	return false
}

// NameFromMetadata generates a name from a package's metadata.
func NameFromMetadata(pkg *v1alpha1.ZarfPackage, isSkeleton bool) string {
	var name string

	arch := config.GetArch(pkg.Metadata.Architecture, pkg.Build.Architecture)

	if isSkeleton {
		arch = zoci.SkeletonArch
	}

	switch pkg.Kind {
	case v1alpha1.ZarfInitConfig:
		name = fmt.Sprintf("zarf-init-%s", arch)
	case v1alpha1.ZarfPackageConfig:
		name = fmt.Sprintf("zarf-package-%s-%s", pkg.Metadata.Name, arch)
	default:
		name = fmt.Sprintf("zarf-%s-%s", strings.ToLower(string(pkg.Kind)), arch)
	}

	if pkg.Build.Differential {
		name = fmt.Sprintf("%s-%s-differential-%s", name, pkg.Build.DifferentialPackageVersion, pkg.Metadata.Version)
	} else if pkg.Metadata.Version != "" {
		name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
	}

	return name
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName() string {
	// No package has been loaded yet so lookup GetArch() with no package info
	arch := config.GetArch()
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// PkgSuffix returns a package suffix based on whether it is uncompressed or not.
func PkgSuffix(uncompressed bool) (suffix string) {
	suffix = ".tar.zst"
	if uncompressed {
		suffix = ".tar"
	}
	return suffix
}
