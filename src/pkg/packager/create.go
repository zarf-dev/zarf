// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/validate"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create() (err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", p.cfg.CreateOpts.BaseDir, err)
	}

	message.Note(fmt.Sprintf("Using build directory %s", p.cfg.CreateOpts.BaseDir))

	c := creator.New(p.cfg.CreateOpts, cwd)

	loadedPkg, warnings, err := c.LoadPackageDefinition(p.layout)
	if err != nil {
		return err
	}

	p.warnings = append(p.warnings, warnings...)

	// Perform early package validation.
	if err := validate.Run(*loadedPkg); err != nil {
		return fmt.Errorf("unable to validate package: %w", err)
	}

	if !utils.ConfirmAction(config.ZarfCreateStage, layout.SBOMDir, []string{}, p.warnings, *loadedPkg, p.cfg.PkgOpts) {
		return fmt.Errorf("package creation canceled")
	}

	if err := c.Assemble(loadedPkg, p.layout); err != nil {
		return err
	}

	// cd back for output
	if err := os.Chdir(cwd); err != nil {
		return err
	}

	return c.Output(loadedPkg, p.layout)
}
