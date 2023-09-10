// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// URLSource is a package source for http, https and sget URLs.
type URLSource struct {
	Destination types.PackagePathsMap
	*types.ZarfPackageOptions
}

// Collect downloads a package from the source URL.
func (s *URLSource) Collect(dstTarball string) error {
	if !config.CommonOptions.Insecure && s.Shasum == "" && !strings.HasPrefix(s.PackageSource, utils.SGETURLPrefix) {
		return fmt.Errorf("remote package provided without a shasum, use --insecure to ignore, or provide one w/ --shasum")
	}
	var packageURL string
	if s.Shasum != "" {
		packageURL = fmt.Sprintf("%s@%s", s.PackageSource, s.Shasum)
	} else {
		packageURL = s.PackageSource
	}

	return utils.DownloadToFile(packageURL, dstTarball, s.SGetKeyPath)
}

// LoadPackage loads a package from an http, https or sget URL.
func (s *URLSource) LoadPackage() (loaded types.PackagePathsMap, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	dstTarball := filepath.Join(tmp, "package.tar.zst")

	if err := s.Collect(dstTarball); err != nil {
		return nil, err
	}

	s.PackageSource = dstTarball

	tp := &TarballSource{
		s.Destination,
		s.ZarfPackageOptions,
	}

	return tp.LoadPackage()
}

// LoadPackageMetadata loads a package's metadata from an http, https or sget URL.
func (s *URLSource) LoadPackageMetadata(wantSBOM bool) (loaded types.PackagePathsMap, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)

	dstTarball := filepath.Join(tmp, "package.tar.zst")

	if err := s.Collect(dstTarball); err != nil {
		return nil, err
	}

	s.PackageSource = dstTarball

	tp := &TarballSource{
		s.Destination,
		s.ZarfPackageOptions,
	}

	return tp.LoadPackageMetadata(wantSBOM)
}
