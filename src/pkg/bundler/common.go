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
	p   packager.Packager
	cfg *types.BundlerConfig
	fs  BundlerFS
	cp  oci.Copier
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

func MergeVariables(left map[string]string, right map[string]string) map[string]string {
	// Ensure uppercase keys from viper and CLI --set
	leftUpper := utils.TransformMapKeys(left, strings.ToUpper)
	rightUpper := utils.TransformMapKeys(right, strings.ToUpper)

	// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
	return utils.MergeMap(leftUpper, rightUpper)
}
