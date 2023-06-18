// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

type Bundler struct {
	pkgr   packager.Packager
	cfg    *types.BundlerConfig
	bundle types.ZarfBundle
	remote *oci.OrasRemote
	fs     BundlerFS
	copier oci.Copier
}

func New(cfg *types.BundlerConfig) (*Bundler, error) {
	message.Debugf("bundler.New(%s)", message.JSONValue(cfg))

	if cfg == nil {
		return nil, ErrBundlerNilConfig
	}

	if cfg.SetVariableMap == nil {
		cfg.SetVariableMap = make(map[string]*types.ZarfSetVariable)
	}

	var (
		err     error
		bundler = &Bundler{
			cfg: cfg,
		}
	)

	if err = bundler.fs.MakeTemp(config.CommonOptions.TempDirectory); err != nil {
		return nil, fmt.Errorf(ErrBundlerUnableToCreateTempDir, err)
	}

	return bundler, nil
}

func NewOrDie(cfg *types.BundlerConfig) *Bundler {
	var (
		err     error
		bundler *Bundler
	)
	if bundler, err = New(cfg); err != nil {
		message.Fatalf(err, ErrBundlerNewOrDie, err)
	}
	return bundler
}

func (b *Bundler) ClearPaths() {
	b.fs.ClearPaths()
}

func (b *Bundler) ValidateBundle() error {
	if b.bundle.Metadata.Version == "" {
		return fmt.Errorf("zarf-bundle.yaml is missing required field: metadata.version")
	}
	if b.bundle.Metadata.Name == "" {
		return fmt.Errorf("zarf-bundle.yaml is missing required field: metadata.name")
	}
	if len(b.bundle.Packages) == 0 {
		return fmt.Errorf("zarf-bundle.yaml is missing required list: packages")
	}
	for idx, pkg := range b.bundle.Packages {
		if pkg.Repository == "" {
			return fmt.Errorf("zarf-bundle.yaml .packages[%d] is missing required field: repository", idx)
		}
		if pkg.Ref == "" {
			return fmt.Errorf("zarf-bundle.yaml .packages[%s] is missing required field: ref", pkg.Repository)
		}
		url := fmt.Sprintf("%s:%s", pkg.Repository, pkg.Ref)
		// validate access to packages as well as components referenced in the package
		remote, err := oci.NewOrasRemote(url)
		if err != nil {
			// remote performs access verification upon instantiation
			return err
		}
		err = remote.PullPackageMetadata(b.fs.tmp.Base)
		if err != nil {
			return err
		}
		defer b.fs.ClearPaths()
		// TODO: validate signatures here
		// TODO: are we gonna re-sign the packages within a bundle?
		requestedComponents := pkg.Components
		if len(requestedComponents) > 0 {
			zarfYAML := types.ZarfPackage{}
			err := utils.ReadYaml(b.fs.tmp.ZarfYaml, &zarfYAML)
			if err != nil {
				return err
			}
			for _, component := range requestedComponents {
				c := utils.Find(zarfYAML.Components, func(c types.ZarfComponent) bool {
					return c.Name == component
				})
				if c.Name == "" {
					return fmt.Errorf("zarf-bundle.yaml .packages[%s].components[%s] does not exist in upstream: %s", pkg.Repository, component, url)
				}
			}
		}
	}
	return nil
}

func MergeVariables(left map[string]string, right map[string]string) map[string]string {
	// Ensure uppercase keys from viper and CLI --set
	leftUpper := utils.TransformMapKeys(left, strings.ToUpper)
	rightUpper := utils.TransformMapKeys(right, strings.ToUpper)

	// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
	return utils.MergeMap(leftUpper, rightUpper)
}
