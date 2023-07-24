// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Remove should do the same as previous code
//
// really this is prob just gonna loop over the packages and call `p.Remove()`
//
// should this support some form of `--components`?
func (b *Bundler) Remove() error {
	ctx := context.TODO()
	// create a new provider
	provider, err := NewProvider(ctx, b.cfg.InspectOpts.Source, b.tmp)
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

	// TODO: support removing bundle by: name / tarball / OCI ref

	return nil
}
