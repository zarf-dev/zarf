// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func (p *Packager) Lint() (err error) {
	zarfSchema, _ := config.GetSchemaFile()
	var zarfData interface{}
	if err := utils.ReadYaml(filepath.Join(p.cfg.CreateOpts.BaseDir, layout.ZarfYAML), &zarfData); err != nil {
		return err
	}

	if err = helpers.ValidateZarfSchema(zarfData, zarfSchema); err != nil {
		return err
	}

	return nil
}
