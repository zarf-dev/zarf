// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	err := p.loadZarfPkg()
	if err != nil {
		return err
	}
	name := fmt.Sprintf("zarf-package-%s-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Metadata.Version)
	err = archiver.Archive([]string{p.tmp.Base}, name)
	if err != nil {
		return err
	}
	return nil
}
