// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	// ocistore "oras.land/oras-go/v2/content/oci"
)

// Inspect pulls/unpacks a bundle's metadata and shows it
//
// : retrieve the `zarf-bundle.yaml`, and `zarf-bundle.yaml.sig`
// : verify sigs
// : show the `zarf-bundle.yaml`
// : have an option to download + persist the SBOMs?
func (b *Bundler) Inspect() error {
	ctx := context.TODO()
	// create a new provider
	provider, err := NewProvider(ctx, b.cfg.InspectOpts.Source, b.tmp)
	if err != nil {
		return err
	}

	// pull the zarf-bundle.yaml + sig
	loaded, err := provider.LoadBundleMetadata()
	if err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	if err := b.ReadBundleYaml(loaded[config.ZarfBundleYAML], &b.bundle); err != nil {
		return err
	}

	// validate the sig (if present)
	if err := ValidateBundleSignature(b.tmp); err != nil {
		return err
	}

	// show the zarf-bundle.yaml
	utils.ColorPrintYAML(b.bundle, nil, false)
	return nil
}
