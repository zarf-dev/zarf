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

type URLProvider struct {
	source         string
	outputTarball  string
	destinationDir string
	opts           *types.ZarfPackageOptions
	insecure       bool
}

func (up *URLProvider) fetchTarball() error {
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

func (up *URLProvider) LoadPackage(optionalComponents []string) (pkg *types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := up.fetchTarball(); err != nil {
		return nil, nil, err
	}

	tp := &TarballProvider{
		source:         up.outputTarball,
		destinationDir: up.destinationDir,
		opts:           up.opts,
	}

	return tp.LoadPackage(optionalComponents)
}

func (up *URLProvider) LoadPackageMetadata(wantSBOM bool) (pkg *types.ZarfPackage, loaded types.PackagePathsMap, err error) {
	if err := up.fetchTarball(); err != nil {
		return nil, nil, err
	}

	tp := &TarballProvider{
		source:         up.outputTarball,
		destinationDir: up.destinationDir,
		opts:           up.opts,
	}

	return tp.LoadPackageMetadata(wantSBOM)
}
