// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

// Remove should do the same as previous code
//
// really this is prob just gonna loop over the packages and call `p.Remove()`
//
// should this support some form of `--components`?
func (b *Bundler) Remove() error {
	return nil
}
