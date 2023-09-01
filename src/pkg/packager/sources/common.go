// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

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

func LoadComponents(pkg *types.ZarfPackage, loaded types.PackagePathsMap) (err error) {
	// always create and "load" components dir
	if _, ok := loaded[types.ComponentsDir]; !ok {
		message.Debugf("Creating %q dir", types.ComponentsDir)
		loaded[types.ComponentsDir] = filepath.Join(loaded[types.BaseDir], types.ComponentsDir)
		if err := utils.CreateDirectory(loaded[types.ComponentsDir], 0755); err != nil {
			return err
		}
	}

	// unpack component tarballs
	for _, component := range pkg.Components {
		tb := filepath.Join(types.ComponentsDir, fmt.Sprintf("%s.tar", component.Name))
		if _, ok := loaded[tb]; ok {
			message.Debugf("Unarchiving %q", tb)
			defer os.Remove(loaded[tb])
			defer delete(loaded, tb)
			if err = archiver.Unarchive(loaded[tb], loaded[types.ComponentsDir]); err != nil {
				return err
			}
		}

		// also "load" the images dir if any component has images
		if _, ok := loaded[types.ImagesDir]; !ok && len(component.Images) > 0 {
			message.Debugf("Creating %q dir", types.ImagesDir)
			loaded[types.ImagesDir] = filepath.Join(loaded[types.BaseDir], types.ImagesDir)
			if err := utils.CreateDirectory(loaded[types.ImagesDir], 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

func LoadSBOMs(loaded types.PackagePathsMap) (err error) {
	// unpack sboms.tar
	if _, ok := loaded[types.SBOMTar]; ok {
		message.Debugf("Unarchiving %q", types.SBOMTar)
		loaded[types.SBOMDir] = filepath.Join(loaded[types.BaseDir], types.SBOMDir)
		if err = archiver.Unarchive(loaded[types.SBOMTar], loaded[types.SBOMDir]); err != nil {
			return err
		}
	} else {
		message.Debug("Package does not contain SBOMs")
	}
	return nil
}
