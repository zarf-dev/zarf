// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

// LoadComponents loads components from a package.
func LoadComponents(pkg *types.ZarfPackage, loaded types.PackagePathsMap) (err error) {
	// always create and "load" components dir
	if !loaded.KeyExists(types.ComponentsDir) {
		message.Debugf("Creating %q dir", types.ComponentsDir)
		if err := loaded.SetDefaultRelative(types.ComponentsDir); err != nil {
			return err
		}
		if err := utils.CreateDirectory(loaded[types.ComponentsDir], 0755); err != nil {
			return err
		}
	}

	// unpack component tarballs
	for _, component := range pkg.Components {
		tb := filepath.Join(types.ComponentsDir, fmt.Sprintf("%s.tar", component.Name))
		if loaded.KeyExists(tb) {
			message.Debugf("Unarchiving %q", tb)
			defer os.Remove(loaded[tb])
			defer delete(loaded, tb)
			if err = archiver.Unarchive(loaded[tb], loaded[types.ComponentsDir]); err != nil {
				return err
			}
		}

		// also "load" the images dir if any component has images
		if !loaded.KeyExists(types.ImagesDir) && len(component.Images) > 0 {
			message.Debugf("Creating %q dir", types.ImagesDir)
			if err := loaded.SetDefaultRelative(types.ImagesDir); err != nil {
				return err
			}
			if err := utils.CreateDirectory(loaded[types.ImagesDir], 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadSBOMs loads SBOMs from a package.
func LoadSBOMs(loaded types.PackagePathsMap) (err error) {
	// unpack sboms.tar
	if loaded.KeyExists(types.SBOMTar) {
		message.Debugf("Unarchiving %q", types.SBOMTar)
		defer os.Remove(loaded[types.SBOMTar])
		defer delete(loaded, types.SBOMTar)
		if err := loaded.SetDefaultRelative(types.SBOMDir); err != nil {
			return err
		}
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
			return err
		}
	} else {
		message.Debug("Package does not contain SBOMs")
	}
	return nil
}
