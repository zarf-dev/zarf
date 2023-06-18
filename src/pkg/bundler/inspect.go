// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Inspect should do the sme as previous code
//
// : retrieve the `zarf-bundle.yaml`, and `zarf.yaml.sig`
// : verify sigs
// : show the `zarf-bundle.yaml`
// : have an option to download + persist the SBOMs?
func (b *Bundler) Inspect() error {
	err := b.SetOCIRemote(b.cfg.InspectOpts.Source)
	if err != nil {
		return err
	}

	// pull the zarf-bundle.yaml + sig
	err = b.remote.PullBundleMetadata(b.fs.tmp.Base)
	if err != nil {
		return err
	}

	// validate the sig (if present)
	err = b.fs.ValidateBundleSignature(b.fs.tmp.Base)
	if err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	err = b.fs.ReadBundleYaml(b.fs.tmp.Base, &b.bundle)
	if err != nil {
		return err
	}
	// show the zarf-bundle.yaml
	utils.ColorPrintYAML(b.bundle)
	return nil
}
