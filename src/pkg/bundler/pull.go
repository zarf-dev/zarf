// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

// Pull should do the same as previous code, except you can pull a package tarball out of a bundle
//
// : retrieve the `zarf-bundle.yaml`, `checksum.txt`, and `zarf.yaml.sig`
// : verify sigs / checksums
// : if `--packages` is specified, pull those packages out of the bundle
// : otherwise pull the entire bundle and save to a bundle tarball
func (b *Bundler) Pull() error {
	return nil
}
