// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

// Pull pulls a bundle and saves it locally + caches it
//
// : retrieve the `zarf-bundle.yaml`, `checksum.txt`, and `zarf.yaml.sig`
// : verify sigs / checksums
// : pull the bundle and tarball it up
func (b *Bundler) Pull() error {
	if err := b.SetOCIRemote(b.cfg.PullOpts.Source); err != nil {
		return err
	}

	// pull the zarf-bundle.yaml + sig
	if err := b.remote.PullBundleMetadata(b.tmp); err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	if err := b.ReadBundleYaml(b.tmp, &b.bundle); err != nil {
		return err
	}

	// validate the sig (if present)
	if err := b.ValidateBundleSignature(b.tmp); err != nil {
		return err
	}

	// if err := b.remote.PullBundle(b.tmp, config.CommonOptions.OCIConcurrency, &b.bundle); err != nil {
	// 	return err
	// }

	return nil
}
