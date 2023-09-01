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
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() error {
	var err error
	p.source, err = sources.New(&p.cfg.PkgOpts, p.tmp.Base())
	if err != nil {
		return err
	}

	var name string

	// TODO: need to think about better naming logic here depending upon the source type
	// might need to be its own function implemented by each source type
	switch p.source.(type) {
	case *sources.OCISource:
		root, err := p.source.(*sources.OCISource).FetchRoot()
		if err != nil {
			return err
		}
		pkg, err := p.source.(*sources.OCISource).FetchZarfYAML(root)
		if err != nil {
			return err
		}
		if strings.HasSuffix(p.cfg.PkgOpts.PackageSource, oci.SkeletonSuffix) {
			name = fmt.Sprintf("zarf-package-%s-skeleton-%s.tar.zst", pkg.Metadata.Name, pkg.Metadata.Version)
		} else {
			name = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", pkg.Metadata.Name, pkg.Build.Architecture, pkg.Metadata.Version)
		}
	case *sources.TarballSource, *sources.PartialTarballSource, *sources.URLSource:
		// note: this is going to break on SGET because of its weird syntax, as well this will break on
		// URLs that do not end w/ a valid file extension
		name = filepath.Base(p.cfg.PkgOpts.PackageSource)
	}

	output := filepath.Join(p.cfg.PullOpts.OutputDirectory, name)
	_ = os.Remove(output)

	if err := p.source.Collect(output); err != nil {
		return err
	}

	message.Infof("Pulled %q", p.cfg.PkgOpts.PackageSource)

	return nil
}
