// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

// Create creates a bundle
func (b *Bundler) Create() error {

	// cd into base
	if err := os.Chdir(b.cfg.CreateOpts.SourceDirectory); err != nil {
		return err
	}
	// read the bundle's metadata into memory
	if err := b.ReadBundleYaml(ZarfBundleYAML, &b.bundle); err != nil {
		return err
	}

	// replace BNDL_TMPL_* variables
	if err := b.templateBundleYaml(); err != nil {
		return err
	}

	// confirm creation
	if ok := b.confirmBundleCreation(); !ok {
		return fmt.Errorf("bundle creation cancelled")
	}

	// make the bundle's build information
	if err := b.CalculateBuildInfo(); err != nil {
		return err
	}

	// validate bundle / verify access to all repositories
	if err := b.ValidateBundle(); err != nil {
		return err
	}

	// set the remote's reference from the bundle's metadata
	ref, err := oci.ReferenceFromMetadata(b.cfg.CreateOpts.Output, &b.bundle.Metadata, b.bundle.Metadata.Architecture)
	if err != nil {
		return err
	}
	if err := b.SetOCIRemote(ref); err != nil {
		return err
	}

	var signatureBytes []byte

	// sign the bundle if a signing key was provided
	if b.cfg.CreateOpts.SigningKeyPath != "" {
		// write the bundle to disk so we can sign it
		bundlePath := filepath.Join(b.tmp, ZarfBundleYAML)
		if err := b.WriteBundleYaml(bundlePath, &b.bundle); err != nil {
			return err
		}

		getSigCreatePassword := func(_ bool) ([]byte, error) {
			if b.cfg.CreateOpts.SigningKeyPassword != "" {
				return []byte(b.cfg.CreateOpts.SigningKeyPassword), nil
			}
			return interactive.PromptSigPassword()
		}
		// sign the bundle
		signaturePath := filepath.Join(b.tmp, ZarfBundleYAMLSignature)
		signatureBytes, err = utils.CosignSignBlob(bundlePath, signaturePath, b.cfg.CreateOpts.SigningKeyPath, getSigCreatePassword)
		if err != nil {
			return err
		}
	}

	// create + publish the bundle
	return Bundle(b.remote, &b.bundle, signatureBytes)
}

// adapted from p.fillActiveTemplate
func (b *Bundler) templateBundleYaml() error {
	message.Debug("Templating", ZarfBundleYAML, "w/:", message.JSONValue(b.cfg.CreateOpts.SetVariables))

	templateMap := map[string]string{}
	setFromCLIConfig := b.cfg.CreateOpts.SetVariables
	yamlTemplates, err := utils.FindYamlTemplates(&b.bundle, "###ZARF_BNDL_TMPL_", "###")
	if err != nil {
		return err
	}

	for key := range yamlTemplates {
		_, present := setFromCLIConfig[key]
		if !present && !config.CommonOptions.Confirm {
			setVal, err := interactive.PromptVariable(types.ZarfPackageVariable{
				Name:    key,
				Default: "",
			})

			if err == nil {
				setFromCLIConfig[key] = setVal
			} else {
				return err
			}
		} else if !present {
			return fmt.Errorf("template '%s' must be '--set' when using the '--confirm' flag", key)
		}
	}
	for key, value := range setFromCLIConfig {
		templateMap[fmt.Sprintf("###ZARF_BNDL_TMPL_%s###", key)] = value
	}

	templateMap["###ZARF_BNDL_ARCH###"] = b.bundle.Metadata.Architecture

	return utils.ReloadYamlTemplate(&b.bundle, templateMap)
}

// adapted from p.confirmAction
func (b *Bundler) confirmBundleCreation() (confirm bool) {

	message.HeaderInfof("🎁 BUNDLE DEFINITION")
	utils.ColorPrintYAML(b.bundle, nil, false)

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
