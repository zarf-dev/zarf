// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"

	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/types"
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	LoadPackageDefinition(ctx context.Context, src *layout.PackagePaths) (pkg types.ZarfPackage, warnings []string, err error)
	Assemble(ctx context.Context, dst *layout.PackagePaths, components []types.ZarfComponent, arch string) error
	Output(ctx context.Context, dst *layout.PackagePaths, pkg *types.ZarfPackage) error
}
