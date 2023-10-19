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

	if err = p.readZarfYAML(filepath.Join(p.cfg.CreateOpts.BaseDir, layout.ZarfYAML)); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
	}
	fmt.Printf("This is the struct %+v\n", p.cfg.Pkg)
	zarfSchema, _ := config.GetSchemaFile()
	var zarfData interface{}

	if err := utils.ReadYaml(filepath.Join(p.cfg.CreateOpts.BaseDir, layout.ZarfYAML), &zarfData); err != nil {
		return err
	}

	helpers.ValidateZarfSchema(zarfData, zarfSchema)

	return nil
}
