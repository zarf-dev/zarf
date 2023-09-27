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
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() (err error) {
	var name string

	if p.cfg.PkgOpts.OptionalComponents != "" {
		return fmt.Errorf("pull does not support optional components")
	}

	switch p.source.(type) {
	case *sources.OCISource:
		name = "zarf-package-unknown-oci.tar.zst"
	case *sources.SplitTarballSource:
		name = strings.Replace(p.cfg.PkgOpts.PackageSource, ".part000", "", 1)
	case *sources.TarballSource, *sources.URLSource:
		name = filepath.Base(p.cfg.PkgOpts.PackageSource)
		if !config.IsValidFileExtension(name) {
			name = "zarf-package-unknown"
		}
	default:
		return fmt.Errorf("pull only currently supports internal source types, received: %T", p.source)
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

		var kind string

		switch pkg.Kind {
		case types.ZarfInitConfig:
			kind = "init"
		case types.ZarfPackageConfig:
			kind = "package"
		default:
			kind = strings.ToLower(string(pkg.Kind))
		}

		name := fmt.Sprintf("zarf-%s-%s-%s", kind, pkg.Metadata.Name, pkg.Build.Architecture)

		if pkg.Metadata.Version != "" {
			name = fmt.Sprintf("%s-%s", name, pkg.Metadata.Version)
		}

		name = filepath.Join(p.cfg.PullOpts.OutputDirectory, name+filepath.Ext(output))
		if err := os.Rename(output, name); err != nil {
			return err
		}
		output = name
	}

	message.Infof("Pulled %q into %q", p.cfg.PkgOpts.PackageSource, output)

	return nil
}
