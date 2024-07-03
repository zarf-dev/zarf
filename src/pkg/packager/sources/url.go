// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
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
	Src           string
	Shasum        string
	PublicKeyPath string
	SGetKeyPath   string
}

// Collect downloads a package from the source URL.
func (s *URLSource) Collect(_ context.Context, dir string) (string, error) {
	if !config.CommonOptions.Insecure && s.Shasum == "" && !strings.HasPrefix(s.Src, helpers.SGETURLPrefix) {
		return "", fmt.Errorf("remote package provided without a shasum, use --insecure to ignore, or provide one w/ --shasum")
	}
	var packageURL string
	if s.Shasum != "" {
		packageURL = fmt.Sprintf("%s@%s", s.Src, s.Shasum)
	} else {
		packageURL = s.Src
	}

	dstTarball := filepath.Join(dir, "zarf-package-url-unknown")

	if err := utils.DownloadToFile(packageURL, dstTarball, s.SGetKeyPath); err != nil {
		return "", err
	}

	return RenameFromMetadata(dstTarball)
}

// LoadPackage loads a package from an http, https or sget URL.
func (s *URLSource) LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return pkg, nil, err
	}
	defer os.Remove(tmp)

	dstTarball, err := s.Collect(ctx, tmp)
	if err != nil {
		return pkg, nil, err
	}
	ts := &TarballSource{
		Src:           dstTarball,
		Shasum:        "",
		PublicKeyPath: s.PublicKeyPath,
	}
	return ts.LoadPackage(ctx, dst, filter, unarchiveAll)
}

// LoadPackageMetadata loads a package's metadata from an http, https or sget URL.
func (s *URLSource) LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg types.ZarfPackage, warnings []string, err error) {
	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return pkg, nil, err
	}
	defer os.Remove(tmp)

	dstTarball, err := s.Collect(ctx, tmp)
	if err != nil {
		return pkg, nil, err
	}
	ts := &TarballSource{
		Src:           dstTarball,
		Shasum:        "",
		PublicKeyPath: s.PublicKeyPath,
	}
	return ts.LoadPackageMetadata(ctx, dst, wantSBOM, skipValidation)
}
