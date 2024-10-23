// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/creator"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	l := logger.From(p.ctx)

	// Set basedir
	baseDir := p.cfg.CreateOpts.BaseDir
	if err := os.Chdir(baseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", baseDir, err)
	}
	l.Info("using build directory", "baseDir", baseDir)

	// Setup package creator
	pc := creator.NewPackageCreator(p.cfg.CreateOpts, cwd)
	if err := helpers.CreatePathAndCopy(layout.ZarfYAML, p.layout.ZarfYAML); err != nil {
		return err
	}

	// Load package def and store on packager config
	// TODO(mkcp): Migrate to logger
	pkg, warnings, err := pc.LoadPackageDefinition(p.ctx, p.layout)
	if err != nil {
		return err
	}
	p.cfg.Pkg = pkg

	// TODO(mkcp): Interactive isolate with slog
	//			   Maybe just comment this out point at
	if !p.confirmAction(config.ZarfCreateStage, warnings, nil) {
		return fmt.Errorf("package creation canceled")
	}

	// TODO(mkcp): Migrate to logger
	if err := pc.Assemble(p.ctx, p.layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
	}

	// cd back for output
	if err := os.Chdir(cwd); err != nil {
		return err
	}

	// TODO(mkcp): migrate pc.Output to logger
	if err = pc.Output(p.ctx, p.layout, &p.cfg.Pkg); err != nil {
		return err
	}
	return nil
}
