// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
)

// Create creates a bundle
func (b *Bundler) Create() error {
	message.Infof("Creating bundle from %s", b.cfg.CreateOpts.SourceDirectory)

	// cd into base
	if err := b.FS.CD(b.cfg.CreateOpts.SourceDirectory); err != nil {
		return err
	}
	// read zarf-bundle.yaml into memory
	if err := b.FS.ReadBundleYaml(config.ZarfBundleYAML, &b.bundle); err != nil {
		return err
	}
	// validate bundle / verify access to all repositories
	if err := b.ValidateBundle(); err != nil {
		return err
	}

	// validate access to the output directory / OCI ref
	ref, err := oci.ReferenceFromMetadata(b.cfg.CreateOpts.Output, &b.bundle.Metadata, b.bundle.Metadata.Architecture)
	if err != nil {
		return err
	}
	err = b.SetOCIRemote(ref.String())
	if err != nil {
		return err
	}

	// make the bundle's build information
	if err := b.CalculateBuildInfo(); err != nil {
		return err
	}

	// create + publish the bundle
	err = b.remote.Bundle(&b.bundle, b.cfg.CreateOpts.SigningKeyPath, b.cfg.CreateOpts.SigningKeyPassword)
	if err != nil {
		return err
	}
	return nil
}
