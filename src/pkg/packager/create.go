// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/creator"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create(ctx context.Context) error {
	l := logger.From(ctx)
	l.Info("starting package create")
	cStart := time.Now()

	// Begin setup
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := p.cfg

	// Set basedir
	createOpts := cfg.CreateOpts
	baseDir := createOpts.BaseDir
	if err := os.Chdir(baseDir); err != nil {
		return fmt.Errorf("unable to access directory %q: %w", baseDir, err)
	}
	// TODO(mkcp): Remove message on logger release
	message.Note(fmt.Sprintf("Using build directory %s", p.cfg.CreateOpts.BaseDir))
	l.Info("using build directory", "baseDir", baseDir)

	// Setup package creator
	lo := p.layout
	pc := creator.NewPackageCreator(createOpts, cwd)
	if err := helpers.CreatePathAndCopy(layout.ZarfYAML, lo.ZarfYAML); err != nil {
		return err
	}

	// Load package def
	pkg, warnings, err := pc.LoadPackageDefinition(p.ctx, lo)
	if err != nil {
		return err
	}
	//  Store on packager config
	p.cfg.Pkg = pkg
	if len(warnings) > 0 {
		l.Warn("warnings found when loading package definition", "warnings", warnings)
	}
	l.Info("package loaded",
		"kind", pkg.Kind,
		"name", pkg.Metadata.Name,
		"description", pkg.Metadata.Description,
	)

	// TODO(mkcp): Remove interactive on logger release
	if !p.confirmAction(config.ZarfCreateStage, warnings, nil) {
		return fmt.Errorf("package creation canceled")
	}

	aStart := time.Now()
	l.Debug("starting package assembly", "kind", pkg.Kind)
	if err := pc.Assemble(ctx, p.layout, pkg.Components, pkg.Metadata.Architecture); err != nil {
		return err
	}
	l.Debug("done assembling package", "kind", pkg.Kind, "duration", time.Since(aStart))

	// cd back for output
	if err := os.Chdir(cwd); err != nil {
		return err
	}

	if err = pc.Output(ctx, p.layout, &pkg); err != nil {
		return err
	}

	l.Debug("done creating package", "kind", pkg.Kind, "duration", time.Since(cStart))
	return nil
}
