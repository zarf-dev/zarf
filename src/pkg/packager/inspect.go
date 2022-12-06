// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package
func (p *Packager) Inspect(packageName string, includeSBOM bool, outputSBOM string) error {
	if utils.InvalidPath(packageName) {
		return fmt.Errorf("invalid package name: %s", packageName)
	}

	// Extract the archive
	_ = archiver.Extract(packageName, config.ZarfYAML, p.tmp.Base)

	configPath := filepath.Join(p.tmp.Base, config.ZarfYAML)

	// Load the config to get the build version
	if err := p.readYaml(configPath, false); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %w", err)
	}

	message.Infof("The package was built with Zarf CLI version %s\n", p.cfg.Pkg.Build.Version)
	utils.ColorPrintYAML(p.cfg.Pkg)

	if includeSBOM || outputSBOM != "" {
		err := archiver.Extract(packageName, "sboms", p.tmp.Base)
		if err != nil {
			return fmt.Errorf("unable to extract the SBOMs: %w", err)
		}
	}

	// Open a browser to view the SBOM if specified
	if includeSBOM {
		sbom.ViewSBOMFiles(p.tmp)
	}

	// Output the SBOM files into a directory if specified
	if outputSBOM != "" {
		if err := sbom.OutputSBOMFiles(p.tmp, outputSBOM); err != nil {
			return err
		}
	}

	return nil
}
