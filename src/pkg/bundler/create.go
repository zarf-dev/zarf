// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/pterm/pterm"
)

// Create creates a bundle
func (b *Bundler) Create() error {
	message.Infof("Creating bundle from %s", b.cfg.CreateOpts.SourceDirectory)

	// cd into base
	if err := os.Chdir(b.cfg.CreateOpts.SourceDirectory); err != nil {
		return err
	}
	// read zarf-bundle.yaml into memory
	if err := b.ReadBundleYaml(config.ZarfBundleYAML, &b.bundle); err != nil {
		return err
	}

	// TODO: implement p.fillActiveTemplate() from packager/variables.go

	// confirm creation
	if ok := b.confirmBundleCreation(); !ok {
		return fmt.Errorf("bundle creation cancelled")
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
	if err := b.SetOCIRemote(ref); err != nil {
		return err
	}

	// make the bundle's build information
	if err := b.CalculateBuildInfo(); err != nil {
		return err
	}

	// create + publish the bundle
	return b.remote.Bundle(&b.bundle, b.cfg.CreateOpts.SigningKeyPath, b.cfg.CreateOpts.SigningKeyPassword)
}

// adapted from p.confirmAction
func (b *Bundler) confirmBundleCreation() (confirm bool) {

	pterm.Println()
	message.HeaderInfof("🎁 BUNDLE DEFINITION")
	utils.ColorPrintYAML(b.bundle, nil, true)

	message.HorizontalRule()

	// Display prompt if not auto-confirmed
	if config.CommonOptions.Confirm {
		return config.CommonOptions.Confirm
	}

	prompt := &survey.Confirm{
		Message: "Create this Zarf bundle?",
	}

	pterm.Println()

	// Prompt the user for confirmation, on abort return false
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		// User aborted or declined, cancel the action
		return false
	}
	return true
}
