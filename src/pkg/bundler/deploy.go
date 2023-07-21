// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

// Deploy deploys a bundle
//
// : create a new provider
// : pull the zarf-bundle.yaml + sig
// : read the zarf-bundle.yaml into memory
// : validate the sig (if present)
// : loop through each package
// : : load the package into a fresh temp dir
// : : validate the sig (if present)
// : : deploy the package
func (b *Bundler) Deploy() error {
	ctx := context.TODO()

	// create a new provider
	provider, err := NewProvider(ctx, b.cfg.DeployOpts.Source, b.tmp)
	if err != nil {
		return err
	}

	// pull the zarf-bundle.yaml + sig
	loaded, err := provider.LoadBundleMetadata()
	if err != nil {
		return err
	}

	// read the zarf-bundle.yaml into memory
	if err := b.ReadBundleYaml(loaded[config.ZarfBundleYAML], &b.bundle); err != nil {
		return err
	}

	// validate the sig (if present)
	if err := ValidateBundleSignature(b.tmp); err != nil {
		return err
	}

	// TODO: state sharing? variable scoping?

	// deploy each package
	for _, pkg := range b.bundle.Packages {
		sha := strings.Split(pkg.Ref, "@sha256:")[1]
		// TODO: figure out how we want to handle passing --packages to deploy
		// TODO: add a `name` field to the package struct
		if len(b.cfg.DeployOpts.Packages) == 0 || helpers.SliceContains(b.cfg.DeployOpts.Packages, sha) {
			pkgTmp, err := utils.MakeTempDir("") // change this to pkg.Name
			if err != nil {
				return err
			}
			defer os.RemoveAll(pkgTmp)
			_, err = provider.LoadPackage(sha, pkgTmp)
			if err != nil {
				return err
			}
			if err := packager.ValidatePackageSignature(pkgTmp, pkg.PublicKey); err != nil {
				return err
			}
			// TODO: this is where we break packager
			// check if is init-package
			// confirm deployment interactively
			// set variables
			// deploy all the components
		}
	}
	return nil
}
