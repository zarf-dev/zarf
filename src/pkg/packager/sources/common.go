// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"github.com/defenseunicorns/zarf/src/pkg/packager/layout"
	"github.com/defenseunicorns/zarf/src/types"
)

// LoadComponents loads components from a package.
func LoadComponents(pkg *types.ZarfPackage, loaded *layout.PackagePaths) (err error) {
	// unpack component tarballs
	for _, component := range pkg.Components {
		if err := loaded.Components.Unarchive(component); err != nil {
			return err
		}
	}
	return nil
}
