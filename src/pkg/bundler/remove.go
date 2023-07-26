// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Remove should do the same as previous code
//
// really this is prob just gonna loop over the packages and call `p.Remove()`
//
// should this support some form of `--components`?
func (b *Bundler) Remove() error {
	ctx := context.TODO()
	// create a new provider
	provider, err := NewProvider(ctx, b.cfg.RemoveOpts.Source, b.tmp)
	if err != nil {
		return err
	}

	// pull the bundle's metadata + sig
	loaded, err := provider.LoadBundleMetadata()
	if err != nil {
		return err
	}

	// read the bundle's metadata into memory
	if err := utils.ReadYaml(loaded[BundleYAML], &b.bundle); err != nil {
		return err
	}

	for _, pkg := range b.bundle.Packages {
		split := strings.Split(pkg.Repository, "/")
		name := split[len(split)-1]
		pkgTmp, err := os.MkdirTemp(b.tmp, name)
		if err != nil {
			return err
		}
		pkgCfg := types.PackagerConfig{
			PkgOpts: types.ZarfPackageOptions{
				PackagePath: name,
			},
		}
		pkgClient, err := packager.New(&pkgCfg)
		if err != nil {
			return err
		}
		if err := pkgClient.SetTempDirectory(pkgTmp); err != nil {
			return err
		}
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Remove(); err != nil {
			return err
		}
	}

	return nil
}
