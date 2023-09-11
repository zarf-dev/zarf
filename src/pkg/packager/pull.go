// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() (err error) {
	var name string

	// TODO: need to think about better naming logic here depending upon the source type
	// might need to be its own function implemented by each source type
	switch p.source.(type) {
	case *sources.OCISource:
		zref, err := oci.ParseZarfPackageReference(p.cfg.PkgOpts.PackageSource)
		if err != nil {
			return err
		}
		name = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", zref.PackageName, zref.Arch, zref.Version)
	case *sources.TarballSource, *sources.SplitTarballSource, *sources.URLSource:
		// note: this is going to break on SGET because of its weird syntax, as well this will break on
		// URLs that do not end w/ a valid file extension
		name = filepath.Base(p.cfg.PkgOpts.PackageSource)
		if !config.IsValidFileExtension(name) {
			// if the URL is not a valid extension, then name based on source
			// archiver.v4 has utilities to detect the compression/format of an archive based on headers
			// but v3 can only determine based on filename
			// so warn the user they will have to rename the file
			name = "zarf-package-unknown"
			message.Warnf("Unable to determine package name based upon provided source %q", p.cfg.PkgOpts.PackageSource)
		}
	}

	output := filepath.Join(p.cfg.PullOpts.OutputDirectory, name)
	_ = os.Remove(output)

	if err := p.source.Collect(output); err != nil {
		return err
	}

	message.Infof("Pulled %q into %q", p.cfg.PkgOpts.PackageSource, output)

	return nil
}
