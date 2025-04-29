// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive declares commonly used internal archival and compression operations in Zarf
package archive

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

// RenameFromMetadata renames a tarball based on its metadata.
// FIXME(mkcp): Simplify, extract out packager-specific stuff
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

	// FIXME(mkcp): Migrate to mholt/archives
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

func identifyUnknownTarball(path string) (string, error) {
	if helpers.InvalidPath(path) {
		return "", &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if filepath.Ext(path) != "" && isValidFileExtension(path) {
		return path, nil
	} else if filepath.Ext(path) != "" && !isValidFileExtension(path) {
		return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, getValidPackageExtensions())
	}

	// rename to .tar.zst and check if it's a valid tar.zst
	tzst := fmt.Sprintf("%s.tar.zst", path)
	if err := os.Rename(path, tzst); err != nil {
		return "", err
	}
	// FIXME(mkcp): Support with internal/archive
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
	// FIXME(mkcp): Migrate to mholt/archives
	format, err = archiver.ByExtension(tb)
	if err != nil {
		return "", err
	}
	_, ok = format.(*archiver.Tar)
	if ok {
		return tb, nil
	}

	return "", fmt.Errorf("%s is not a supported tarball format (%+v)", path, getValidPackageExtensions())
}

// getValidPackageExtensions returns the valid package extensions.
// NOTE(mkcp): Similar to archives format
func getValidPackageExtensions() [2]string {
	return [...]string{".tar.zst", ".tar"}
}

// IsValidFileExtension returns true if the filename has a valid package extension.
func isValidFileExtension(filename string) bool {
	for _, extension := range getValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			return true
		}
	}

	return false
}

// NameFromMetadata generates a name from a package's metadata.
// FIXME(mkcp) Lots of packager-specific details here, figure out where this lives in packager2.
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
