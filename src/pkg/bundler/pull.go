// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a bundle and saves it locally + caches it
//
// : retrieve the `zarf-bundle.yaml`, `index.json`, and `zarf.yaml.sig`
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

	// TODO: caching???

	// load the bundle into memory
	if err := b.ReadBundleYaml(filepath.Join(b.tmp, config.ZarfBundleYAML), &b.bundle); err != nil {
		return err
	}

	// tarball the bundle
	filename := fmt.Sprintf("zarf-bundle-%s-%s-%s.tar.zst", b.bundle.Metadata.Name, b.bundle.Metadata.Architecture, b.bundle.Metadata.Version)
	dst := filepath.Join(b.cfg.PullOpts.OutputDirectory, filename)

	_ = os.RemoveAll(dst)

	if err := archiver.Archive([]string{b.tmp + string(os.PathSeparator)}, dst); err != nil {
		return err
	}

	message.Debug("Bundle tarball saved to", dst)

	return nil
}
