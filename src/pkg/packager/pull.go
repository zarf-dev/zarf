// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	var err error
	p.source, err = sources.New(&p.cfg.PkgOpts, p.tmp.Base())
	if err != nil {
		return err
	}

	// TODO: figure out either a new fn (PullPackage?) or a way to "load" w/o unpacking tarballs
	pkg, loaded, err := p.source.LoadPackage(nil)
	if err != nil {
		return err
	}
	p.cfg.Pkg = pkg

	message.Infof("Pulled %q", p.cfg.PkgOpts.PackageSource)

	// Get all the files loaded
	everything := []string{}
	for _, layer := range loaded {
		everything = append(everything, layer)
	}

	var name string
	if strings.HasSuffix(p.cfg.PkgOpts.PackageSource, oci.SkeletonSuffix) {
		name = fmt.Sprintf("zarf-package-%s-skeleton-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Metadata.Version)
	} else {
		name = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", p.cfg.Pkg.Metadata.Name, p.cfg.Pkg.Build.Architecture, p.cfg.Pkg.Metadata.Version)
	}
	output := filepath.Join(p.cfg.PullOpts.OutputDirectory, name)
	_ = os.Remove(output)
	err = archiver.Archive(everything, output)
	if err != nil {
		return err
	}
	return nil
}
