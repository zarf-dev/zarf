// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/types"
)

type Bundler struct {
	p   packager.Packager
	cfg *types.BundlerConfig
	fs  BundlerFS
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
		message.Fatalf(err, "Unable to setup the bundler config: %s", err.Error())
	}
	return bundler
}
