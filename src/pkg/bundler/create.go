// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
)

func (b *Bundler) Create() error {
	message.Infof("Creating bundle from %s", b.cfg.CreateOpts.SourceDirectory)

	// validate access to the output directory / OCI ref
	ref, err := oci.ReferenceFromMetadata(b.cfg.CreateOpts.Output, &b.bundle.Metadata, b.bundle.Metadata.Architecture)
	if err != nil {
		return err
	}
	remote, err := oci.NewOrasRemote(ref.String())
	if err != nil {
		return err
	}
	b.remote = remote

	// cd into base
	if err := b.fs.CD(b.cfg.CreateOpts.SourceDirectory); err != nil {
		return err
	}
	// read zarf-bundle.yaml into memory
	if err := b.fs.ReadBundleYaml(config.ZarfBundleYAML, &b.bundle); err != nil {
		return err
	}
	// validate bundle / verify access to all repositories
	if err := b.ValidateBundle(); err != nil {
		return err
	}

	// create + publish the bundle
	err = b.remote.Bundle(&b.bundle, b.cfg.CreateOpts.SigningKeyPath, b.cfg.CreateOpts.SigningKeyPassword)
	if err != nil {
		return err
	}
	return nil
}
