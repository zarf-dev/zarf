// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/creator"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create(ctx context.Context) (err error) {
	message.Note(fmt.Sprintf("Using build directory %s", p.cfg.CreateOpts.BaseDir))
	pc := creator.NewPackageCreator(p.cfg.CreateOpts)
	if err := helpers.CreatePathAndCopy(filepath.Join(p.cfg.CreateOpts.BaseDir, layout.ZarfYAML), p.layout.ZarfYAML); err != nil {
		return err
	}
	p.cfg.Pkg, p.warnings, err = pc.LoadPackageDefinition(ctx, p.layout)
	if err != nil {
		return err
	}
	if !p.confirmAction(config.ZarfCreateStage) {
		return fmt.Errorf("package creation canceled")
	}
	if err := pc.Assemble(ctx, p.layout, p.cfg.Pkg.Components, p.cfg.Pkg.Metadata.Architecture); err != nil {
		return err
	}
	return pc.Output(ctx, p.layout, &p.cfg.Pkg)
}
