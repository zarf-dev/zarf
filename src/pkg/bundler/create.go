// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import "github.com/defenseunicorns/zarf/src/pkg/message"

func (b *Bundler) Create() error {
	// cd into base
	// read zarf-bundle.yaml into memory
	// create remotes for all repositories
	// ^ verify access to all repositories
	// create the manifest.json by mergin all the manifests + de-duping image layers
	// create the BundlerFS out of this manifest
	// blob mount any needed external blobs
	// otherwise just copy the blobs
	message.Infof("Creating %s", b.cfg.CreateOpts.SourceDirectory)
	return nil
}
