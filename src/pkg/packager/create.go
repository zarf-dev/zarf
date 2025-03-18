// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"

	"github.com/zarf-dev/zarf/src/internal/packager2"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Create(ctx context.Context) error {
	createOpt := packager2.CreateOptions{
		Flavor:                  p.cfg.CreateOpts.Flavor,
		RegistryOverrides:       p.cfg.CreateOpts.RegistryOverrides,
		SigningKeyPath:          p.cfg.CreateOpts.SigningKeyPath,
		SigningKeyPassword:      p.cfg.CreateOpts.SigningKeyPassword,
		SetVariables:            p.cfg.CreateOpts.SetVariables,
		MaxPackageSizeMB:        p.cfg.CreateOpts.MaxPackageSizeMB,
		SBOMOut:                 p.cfg.CreateOpts.SBOMOutputDir,
		SkipSBOM:                p.cfg.CreateOpts.SkipSBOM,
		Output:                  p.cfg.CreateOpts.Output,
		DifferentialPackagePath: p.cfg.CreateOpts.DifferentialPackagePath,
	}
	return packager2.Create(ctx, p.cfg.CreateOpts.BaseDir, createOpt)
}
