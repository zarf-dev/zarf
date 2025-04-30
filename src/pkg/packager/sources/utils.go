// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
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

func identifyUnknownTarball(path string) (string, error) {
	if helpers.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if filepath.Ext(path) != "" && IsValidFileExtension(path) {
		return path, nil
	} else if filepath.Ext(path) != "" && !IsValidFileExtension(path) {
		return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, GetValidPackageExtensions())
	}

	// rename to .tar.zst and check if it's a valid tar.zst
	tzst := fmt.Sprintf("%s.tar.zst", path)
	if err := os.Rename(path, tzst); err != nil {
		return "", err
	}
	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	format, err := archiver.ByExtension(tzst)
	if err != nil {
		return "", err
	}
	_, ok := format.(*archiver.TarZstd)
	if ok {
		return tzst, nil
	}

	// rename to .tar and check if it's a valid tar
	tb := fmt.Sprintf("%s.tar", path)
	if err := os.Rename(tzst, tb); err != nil {
		return "", err
	}
	format, err = archiver.ByExtension(tb)
	if err != nil {
		return "", err
	}
	_, ok = format.(*archiver.Tar)
	if ok {
		return tb, nil
	}

	return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, GetValidPackageExtensions())
}

// RenameFromMetadata renames a tarball based on its metadata.
func RenameFromMetadata(path string) (string, error) {
	var pkg v1alpha1.ZarfPackage

	ext := filepath.Ext(path)
	if ext == "" {
		pathWithExt, err := identifyUnknownTarball(path)
		if err != nil {
			return "", err
		}
		path = pathWithExt
		ext = filepath.Ext(path)
	}
	if ext == ".zst" {
		ext = ".tar.zst"
	}

	// TODO(mkcp): See https://github.com/zarf-dev/zarf/issues/3051
	if err := archiver.Walk(path, func(f archiver.File) error {
		if f.Name() == layout.ZarfYAML {
			b, err := io.ReadAll(f)
			if err != nil {
				return err
			}
			if err := goyaml.Unmarshal(b, &pkg); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return "", err
	}

	if pkg.Metadata.Name == "" {
		return "", fmt.Errorf("%q does not contain a zarf.yaml", path)
	}

	name := NameFromMetadata(&pkg, false)

	name = fmt.Sprintf("%s%s", name, ext)

	tb := filepath.Join(filepath.Dir(path), name)

	return tb, os.Rename(path, tb)
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
