// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	LoadPackageDefinition() error
	ComposeComponents() (warnings []string, err error)
	FillActiveTemplate() (warnings []string, err error)
	ProcessExtensions() error
	LoadDifferentialData() error
	RemoveCopiesFromDifferentialPackage() error
}

// New returns a new Creator based on the provided create options.
func New(createOpts *types.ZarfCreateOptions) (Creator, error) {
	sc := &SkeletonCreator{}
	pc := &PackageCreator{}

	if createOpts.IsSkeleton {
		// If the temp directory is not set, set it to the default
		if sc.layout == nil {
			if err := sc.setTempDirectory(config.CommonOptions.TempDirectory); err != nil {
				return nil, fmt.Errorf("unable to create package temp paths: %w", err)
			}
		}
		return sc, nil
	}

	// If the temp directory is not set, set it to the default
	if pc.layout == nil {
		if err := pc.setTempDirectory(config.CommonOptions.TempDirectory); err != nil {
			return nil, fmt.Errorf("unable to create package temp paths: %w", err)
		}
	}
	return pc, nil
}
