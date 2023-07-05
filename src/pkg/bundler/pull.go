// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a bundle and saves it locally + caches it
//
// : retrieve the `zarf-bundle.yaml`, `checksum.txt`, and `zarf.yaml.sig`
// : verify sigs / checksums
// : pull the bundle and tarball it up
func (b *Bundler) Pull() error {
	if err := b.SetOCIRemote(b.cfg.PullOpts.Source); err != nil {
		return err
	}

	// TODO: figure out the best path to check the signature before we pull the bundle

	// pull the bundle
	if err := b.remote.PullBundle(b.tmp, config.CommonOptions.OCIConcurrency, nil); err != nil {
		return err
	}

	// tarball the bundle
	if err := archiver.Archive([]string{b.tmp + string(os.PathSeparator)}, b.cfg.PullOpts.OutputDirectory); err != nil {
		return err
	}

	return nil
}
