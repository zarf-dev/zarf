// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() (err error) {
	var name string

	switch p.source.(type) {
	case *sources.OCISource:
		zref, err := oci.ParseZarfPackageReference(p.cfg.PkgOpts.PackageSource)
		if err != nil {
			return err
		}
		name = fmt.Sprintf("zarf-package-%s-%s-%s.tar.zst", zref.PackageName, zref.Arch, zref.Version)
	case *sources.SplitTarballSource:
		name = strings.Replace(p.cfg.PkgOpts.PackageSource, ".part000", "", 1)
	case *sources.TarballSource, *sources.URLSource:
		name = filepath.Base(p.cfg.PkgOpts.PackageSource)
		if !config.IsValidFileExtension(name) {
			name = "zarf-package-unknown"
		}
	}

	output := filepath.Join(p.cfg.PullOpts.OutputDirectory, name)
	_ = os.Remove(output)

	if err := p.source.Collect(output); err != nil {
		return err
	}

	if !config.IsValidFileExtension(output) {
		output, err = sources.TransformUnkownTarball(output)
		if err != nil {
			return err
		}
		var pkg types.ZarfPackage
		if err := archiver.Walk(output, func(f archiver.File) error {
			if f.Name() == layout.ZarfYAML {
				b, err := io.ReadAll(f)
				if err != nil {
					return err
				}
				if err := goyaml.Unmarshal(b, &pkg); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		newName := fmt.Sprintf("zarf-package-%s-%s", pkg.Metadata.Name, pkg.Build.Architecture)

		if pkg.Metadata.Version != "" {
			newName = fmt.Sprintf("%s-%s", newName, pkg.Metadata.Version)
		}

		newName = newName + filepath.Ext(output)
		if err := os.Rename(output, newName); err != nil {
			return err
		}
		output = newName
	}

	message.Infof("Pulled %q into %q", p.cfg.PkgOpts.PackageSource, output)

	return nil
}
