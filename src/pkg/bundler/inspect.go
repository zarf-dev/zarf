// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

// Inspect should do the sme as previous code
//
// : retrieve the `zarf-bundle.yaml`, `checksum.txt`, and `zarf.yaml.sig`
// : verify sigs / checksums
// : show the `zarf-bundle.yaml`
// : have an option to download + persist the SBOMs
func (b *Bundler) Inspect() error {
	return nil
}
