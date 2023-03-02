// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path/filepath"

	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	err := p.loadZarfPkg()
	if err != nil {
		return err
	}

	// Get all the layers from within the temp directory
	allTheLayers, err := filepath.Glob(filepath.Join(p.tmp.Base, "*"))
	if err != nil {
		return err
	}

	name := fmt.Sprintf("zarf-package-%s-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Metadata.Version)
	err = archiver.Archive(allTheLayers, name)
	if err != nil {
		return err
	}
	return nil
}
