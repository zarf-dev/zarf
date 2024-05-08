// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that URLSource implements PackageSource
	_ PackageSource = (*URLSource)(nil)
)

// URLSource is a package source for http, https and sget URLs.
type URLSource struct {
	*types.ZarfPackageOptions
}

// Collect downloads a package from the source URL.
func (s *URLSource) Collect(dir string) (string, error) {
	if !config.CommonOptions.Insecure && s.Shasum == "" && !strings.HasPrefix(s.PackageSource, helpers.SGETURLPrefix) {
		return "", fmt.Errorf("remote package provided without a shasum, use --insecure to ignore, or provide one w/ --shasum")
	}
	var packageURL string
	if s.Shasum != "" {
		packageURL = fmt.Sprintf("%s@%s", s.PackageSource, s.Shasum)
	} else {
		packageURL = s.PackageSource
	}

	dstTarball := filepath.Join(dir, "zarf-package-url-unknown")

	if err := utils.DownloadToFile(packageURL, dstTarball, s.SGetKeyPath); err != nil {
		return "", err
	}

	return RenameFromMetadata(dstTarball)
}

// LoadPackage loads a package from an http, https or sget URL.
func (s *URLSource) LoadPackage(dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return pkg, nil, err
	}
	defer os.Remove(tmp)

	dstTarball, err := s.Collect(tmp)
	if err != nil {
		return pkg, nil, err
	}

	s.PackageSource = dstTarball
	// Clear the shasum so that it doesn't get used again
	s.Shasum = ""

	ts := &TarballSource{
		s.ZarfPackageOptions,
	}

	return ts.LoadPackage(dst, filter, unarchiveAll)
}

// LoadPackageMetadata loads a package's metadata from an http, https or sget URL.
func (s *URLSource) LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return pkg, nil, err
	}
	defer os.Remove(tmp)

	dstTarball, err := s.Collect(tmp)
	if err != nil {
		return pkg, nil, err
	}

	s.PackageSource = dstTarball

	ts := &TarballSource{
		s.ZarfPackageOptions,
	}

	return ts.LoadPackageMetadata(dst, wantSBOM, skipValidation)
}
