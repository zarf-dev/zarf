// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import "github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"

func (p *Packager) runMigrations() {
	var warnings []string

	if p.layout.IsLegacyLayout() {
		warning := "Detected deprecated package layout, migrating to new layout - support for this package will be dropped in v1.0.0"
		p.warnings = append(p.warnings, warning)
	}

	if len(p.cfg.Pkg.Build.Migrations) > 0 {
		for idx, component := range p.cfg.Pkg.Components {
			// Handle component configuration deprecations
			p.cfg.Pkg.Components[idx], warnings = deprecated.MigrateComponent(p.cfg.Pkg.Build, component)
			p.warnings = append(p.warnings, warnings...)
		}
	}
}
