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
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a bundle and saves it locally + caches it
//
// : retrieve the `zarf-bundle.yaml`, and `zarf-bundle.yaml.sig`
// : verify sigs / checksums
// : pull the bundle into cache and tarball it up
func (b *Bundler) Pull() error {
	if err := b.SetOCIRemote(b.cfg.PullOpts.Source); err != nil {
		return err
	}

	// fetch the bundle's root descriptor
	// to later get the bundle's descriptor
	root, err := b.remote.FetchRoot()
	if err != nil {
		return err
	}

	cacheDir := filepath.Join(config.GetAbsCachePath(), "packages")

	if utils.InvalidPath(cacheDir) {
		if err := utils.CreateDirectory(cacheDir, 0755); err != nil {
			return err
		}
	}

	// TODO: figure out the best path to check the signature before we pull the bundle

	// pull the bundle
	if err := b.remote.PullBundle(cacheDir, config.CommonOptions.OCIConcurrency, nil); err != nil {
		return err
	}

	// locate the zarf-bundle.yaml's descriptor
	bundleDesc := root.Locate(config.ZarfBundleYAML)
	if err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	bundleYamlPath := filepath.Join(cacheDir, "blobs", "sha256", bundleDesc.Digest.Encoded())

	// load the bundle into memory
	if err := b.ReadBundleYaml(bundleYamlPath, &b.bundle); err != nil {
		return err
	}

	// tarball the bundle
	filename := fmt.Sprintf("zarf-bundle-%s-%s-%s.tar.zst", b.bundle.Metadata.Name, b.bundle.Metadata.Architecture, b.bundle.Metadata.Version)
	dst := filepath.Join(b.cfg.PullOpts.OutputDirectory, filename)

	// TODO: instead of removing then writing
	// 	 we should figure out a way to stream the
	//   differential into the tarball

	_ = os.RemoveAll(dst)

	if err := archiver.Archive([]string{b.tmp + string(os.PathSeparator)}, dst); err != nil {
		return err
	}

	message.Debug("Bundle tarball saved to", dst)

	return nil
}
