// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// URLProvider is a package provider for http, https and sget URLs.
type URLProvider struct {
	source         string
	outputTarball  string
	destinationDir string
	opts           *types.ZarfPackageOptions
	insecure       bool
}

// fetchTarball downloads the tarball from the URL.
func (up *URLProvider) fetchTarball() error {
	// TODO: do we want to support caching if the SHA is provided?

	if !up.insecure && up.opts.Shasum == "" {
		return fmt.Errorf("remote package provided without a shasum, use --insecure to ignore, or provide one w/ --shasum")
	}
	var packageURL string
	if up.opts.Shasum != "" {
		packageURL = fmt.Sprintf("%s@%s", up.source, up.opts.Shasum)
	} else {
		packageURL = up.source
	}

	tmp, err := utils.MakeTempDir()
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	dstTarball := filepath.Join(tmp, "package.tar.zst")

	if err := utils.DownloadToFile(packageURL, dstTarball, up.opts.SGetKeyPath); err != nil {
		return err
	}

	up.outputTarball = dstTarball
	return nil
}

// LoadPackage loads a package from an http, https or sget URL.
func (up *URLProvider) LoadPackage(optionalComponents []string) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := up.fetchTarball(); err != nil {
		return pkg, nil, err
	}

	tp := &TarballProvider{
		source:         up.outputTarball,
		destinationDir: up.destinationDir,
		opts:           up.opts,
	}

	return tp.LoadPackage(optionalComponents)
}

// LoadPackageMetadata loads a package's metadata from an http, https or sget URL.
func (up *URLProvider) LoadPackageMetadata(wantSBOM bool) (pkg types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := up.fetchTarball(); err != nil {
		return pkg, nil, err
	}

	tp := &TarballProvider{
		source:         up.outputTarball,
		destinationDir: up.destinationDir,
		opts:           up.opts,
	}

	return tp.LoadPackageMetadata(wantSBOM)
}
