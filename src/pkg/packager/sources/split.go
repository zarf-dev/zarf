// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that SplitTarballSource implements PackageSource
	_ PackageSource = (*SplitTarballSource)(nil)
)

// SplitTarballSource is a package source for split tarballs.
type SplitTarballSource struct {
	*types.ZarfPackageOptions
}

// Collect turns a split tarball into a full tarball.
func (s *SplitTarballSource) Collect(_ context.Context, dir string) (string, error) {
	pattern := strings.Replace(s.PackageSource, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("unable to find split tarball files: %s", err)
	}

	// Ensure the files are in order so they are appended in the correct order
	sort.Strings(fileList)

	reassembled := filepath.Join(dir, filepath.Base(strings.Replace(s.PackageSource, ".part000", "", 1)))
	// Create the new package
	pkgFile, err := os.Create(reassembled)
	if err != nil {
		return "", fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	var pkgData types.ZarfSplitPackageData
	for idx, file := range fileList {
		// The first file contains metadata about the package
		if idx == 0 {
			var bytes []byte

			if bytes, err = os.ReadFile(file); err != nil {
				return "", fmt.Errorf("unable to read file %s: %w", file, err)
			}

			if err := json.Unmarshal(bytes, &pkgData); err != nil {
				return "", fmt.Errorf("unable to unmarshal file %s: %w", file, err)
			}

			count := len(fileList) - 1
			if count != pkgData.Count {
				return "", fmt.Errorf("package is missing parts, expected %d, found %d", pkgData.Count, count)
			}

			if len(s.Shasum) > 0 && pkgData.Sha256Sum != s.Shasum {
				return "", fmt.Errorf("mismatch in CLI options and package metadata, expected %s, found %s", s.Shasum, pkgData.Sha256Sum)
			}

			continue
		}

		// Open the file
		f, err := os.Open(file)
		if err != nil {
			return "", fmt.Errorf("unable to open file %s: %w", file, err)
		}
		defer f.Close()

		// Add the file contents to the package
		if _, err = io.Copy(pkgFile, f); err != nil {
			return "", fmt.Errorf("unable to copy file %s: %w", file, err)
		}

		// Close the file when done copying
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("unable to close file %s: %w", file, err)
		}
	}

	if err := helpers.SHAsMatch(reassembled, pkgData.Sha256Sum); err != nil {
		return "", fmt.Errorf("package integrity check failed: %w", err)
	}

	// Remove the parts to reduce disk space before extracting
	for _, file := range fileList {
		_ = os.Remove(file)
	}

	// communicate to the user that the package was reassembled
	message.Infof("Reassembled package to: %q", reassembled)

	return reassembled, nil
}

// LoadPackage loads a package from a split tarball.
func (s *SplitTarballSource) LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tb, err := s.Collect(ctx, filepath.Dir(s.PackageSource))
	if err != nil {
		return pkg, nil, err
	}

	// Update the package source to the reassembled tarball
	s.PackageSource = tb
	// Clear the shasum so it is not used for validation
	s.Shasum = ""

	ts := &TarballSource{
		s.ZarfPackageOptions,
	}
	return ts.LoadPackage(ctx, dst, filter, unarchiveAll)
}

// LoadPackageMetadata loads a package's metadata from a split tarball.
func (s *SplitTarballSource) LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tb, err := s.Collect(ctx, filepath.Dir(s.PackageSource))
	if err != nil {
		return pkg, nil, err
	}

	// Update the package source to the reassembled tarball
	s.PackageSource = tb

	ts := &TarballSource{
		s.ZarfPackageOptions,
	}
	return ts.LoadPackageMetadata(ctx, dst, wantSBOM, skipValidation)
}
