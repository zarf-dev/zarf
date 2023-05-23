// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
)

func (p *Packager) confirmAction(userMessage string, sbomViewFiles []string, packageWarnings []string) (confirm bool) {

	message.HorizontalRule()
	utils.ColorPrintYAML(p.cfg.Pkg)

	// Print the location that the user can view the package SBOMs from
	if len(sbomViewFiles) > 0 {
		cwd, _ := os.Getwd()
		link := pterm.FgWhite.Sprint(pterm.Bold.Sprint(filepath.Join(cwd, config.ZarfSBOMDir, filepath.Base(sbomViewFiles[0]))))
		msg := fmt.Sprintf("This package has %d artifacts with software bill-of-materials (SBOM) included. You can view them now in the zarf-sbom folder in this directory or to go directly to one, open this in your browser: %s", len(sbomViewFiles), link)
		message.HorizontalNoteRule()
		message.Note(msg)
		message.Note(" * This directory will be removed after package deployment.")
	}

	// Print any potential breaking changes (if this is a Deploy confirm) between this CLI version and the deployed init package
	if userMessage == "Deploy" {
		if cluster, err := cluster.NewCluster(); err == nil {
			if initPackage, err := cluster.GetDeployedPackage("init"); err == nil {
				// We use the build.version for now because it is the most reliable way to get this version info pre v0.26.0
				deprecated.PrintBreakingChanges(initPackage.Data.Build.Version)
			}
		}

		message.HorizontalNoteRule()
	}

	if len(packageWarnings) > 0 {
		for _, warning := range packageWarnings {
			message.Warn(warning)
		}
		message.HorizontalNoteRule()
	}

	// Display prompt if not auto-confirmed
	if config.CommonOptions.Confirm {
		message.Successf("%s Zarf package confirmed", userMessage)
		return config.CommonOptions.Confirm
	}

	prompt := &survey.Confirm{
		Message: userMessage + " this Zarf package?",
	}

	pterm.Println()

	// Prompt the user for confirmation, on abort return false
	if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
		// User aborted or declined, cancel the action
		return false
	}

	// On create in interactive mode, prompt for max package size if it is still the default value of 0
	// Note: it will not be 0 if the user has provided a value via the --max-package-size flag or Viper config
	if userMessage == "Create" && p.cfg.CreateOpts.MaxPackageSizeMB == 0 {
		value, err := p.promptVariable(types.ZarfPackageVariable{
			Name:        "Maximum Package Size",
			Description: "Specify a maximum file size for this package in Megabytes. Above this size, the package will be split into multiple files. 0 will disable this feature.",
			Default:     "0",
		})
		if err != nil {
			// User aborted, cancel the action
			return false
		}

		// Try to parse the value, on error warn and move on
		maxPackageSize, err := strconv.Atoi(value)
		if err != nil {
			message.Warnf("Unable to parse \"%s\" as a number for the maximum file size. This package will not be split into multiple files.", value)
			return true
		}

		p.cfg.CreateOpts.MaxPackageSizeMB = maxPackageSize
	}

	return true
}

func (p *Packager) promptVariable(variable types.ZarfPackageVariable) (value string, err error) {

	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for \"%s\"", variable.Name),
		Default: variable.Default,
	}

	if err = survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
