// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archives"
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

// identifyUnknownTarball tries "path" as-is first, then retries
// with .tar.zst, .tar.gz, .tar.xz, and .tar appended,
// using archives.Identify to detect only tar variants.
func identifyUnknownTarball(path string) (string, error) {
	// 1) missing file?
	if helpers.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	ctx := context.Background()

	// helper to test a candidate filename
	try := func(name string) (bool, error) {
		f, err := os.Open(name)
		if err != nil {
			return false, err
		}
		defer f.Close()

		// Identify by filename or header
		format, _, err := archives.Identify(ctx, filepath.Base(name), f)
		if err != nil {
			// NoMatch or other error
			return false, nil
		}

		// "format" might be a plain Tar, or a CompressedArchive wrapping Tar
		switch v := format.(type) {
		case archives.Tar:
			return true, nil
		case archives.CompressedArchive:
			if _, ok := v.Archival.(archives.Tar); ok {
				return true, nil
			}
		}
		return false, nil
	}

	// 2) try original path
	if _, err := try(path); err != nil {
		return "", err
	}
	// else if ok {
	// 	return path, nil
	// }

	// 3) try each extension in order
	for _, ext := range []string{".tar.zst", ".tar.gz", ".tar.xz", ".tar"} {
		newPath := path + ext
		if err := os.Rename(path, newPath); err != nil {
			continue // maybe file locked or already renamed
		}

		if ok, err := try(newPath); err != nil {
			// rename back before bailing
			_ = os.Rename(newPath, path)
			return "", err
		} else if ok {
			return newPath, nil
		}

		// not a tar variant—roll back rename
		_ = os.Rename(newPath, path)
	}

	return "", fmt.Errorf("%s is not a supported tarball format (%v)",
		path, GetValidPackageExtensions())
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

	fsys, err := archives.FileSystem(context.Background(), path, nil)
	if err != nil {
		return "", fmt.Errorf("unable to open archive %q: %w", path, err)
	}

	// 3) open just the zarf.yaml entry
	f, err := fsys.Open(layout.ZarfYAML)
	if err != nil {
		return "", fmt.Errorf("%s does not contain a %s", path, layout.ZarfYAML)
	}

	// 4) read & unmarshal into our package struct
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	if err := goyaml.Unmarshal(data, &pkg); err != nil {
		return "", err
	}

	if pkg.Metadata.Name == "" {
		return "", fmt.Errorf("%q does not contain a zarf.yaml", path)
	}

	name := NameFromMetadata(&pkg, false)

	name = fmt.Sprintf("%s%s", name, ext)

	tb := filepath.Join(filepath.Dir(path), name)

	// Windows will not allow the rename if open
	err = f.Close()
	if err != nil {
		return "", err
	}

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
