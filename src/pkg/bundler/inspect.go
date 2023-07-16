// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Inspect pulls/unpacks a bundle's metadata and shows it
//
// : retrieve the `zarf-bundle.yaml`, and `zarf-bundle.yaml.sig`
// : verify sigs
// : show the `zarf-bundle.yaml`
// : have an option to download + persist the SBOMs?
func (b *Bundler) Inspect() error {
	processor, err := NewProcessor(b.cfg.InspectOpts.Source)
	if err != nil {
		return err
	}

	// pull the zarf-bundle.yaml + sig
	if err := processor.LoadBundleMetadata(b.tmp); err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	if err := b.ReadBundleYaml(filepath.Join(b.tmp, config.ZarfBundleYAML), &b.bundle); err != nil {
		return err
	}

	// validate the sig (if present)
	if err := b.ValidateBundleSignature(b.tmp); err != nil {
		return err
	}

	// show the zarf-bundle.yaml
	utils.ColorPrintYAML(b.bundle, nil, false)
	return nil
}
