// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	// ocistore "oras.land/oras-go/v2/content/oci"
)

// Inspect pulls/unpacks a bundle's metadata and shows it
func (b *Bundler) Inspect() error {
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
	if err := b.ReadBundleYaml(loaded[BundleYAML], &b.bundle); err != nil {
		return err
	}

	// validate the sig (if present)
	if err := ValidateBundleSignature(b.tmp); err != nil {
		return err
	}

	// show the bundle's metadata
	utils.ColorPrintYAML(b.bundle, nil, false)

	// TODO: showing SBOMs?
	// TODO: showing package metadata?
	// TODO: could be cool to have an interactive mode that lets you select a package and show its metadata
	return nil
}
