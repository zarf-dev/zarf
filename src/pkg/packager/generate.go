// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

func UpdatePackageConfig(pkgConfig *types.PackagerConfig) types.PackagerConfig {

	// Add new ZarfPackageConfig
	newComponent := []types.ZarfComponent{{
		Name:     pkgConfig.GenerateOpts.Name,
		Required: true,
		Charts: []types.ZarfChart{
			{
				Name:      pkgConfig.GenerateOpts.Name,
				Version:   pkgConfig.GenerateOpts.Version,
				Namespace: pkgConfig.GenerateOpts.Name,
				URL:       pkgConfig.GenerateOpts.URL,
				GitPath:   pkgConfig.GenerateOpts.GitPath,
			},
		},
	}}

	newPkg := pkgConfig.Pkg
	newPkg.Kind = "ZarfPackageConfig"
	newPkg.Metadata.Name = pkgConfig.GenerateOpts.Name
	newPkg.Components = append(newComponent, newPkg.Components...)
	pkgConfig.Pkg = newPkg

	// Set config for FindImages and helm
	pkgConfig.CreateOpts.BaseDir = "."
	pkgConfig.FindImagesOpts.RepoHelmChartPath = pkgConfig.GenerateOpts.GitPath

	v := common.GetViper()
	pkgConfig.CreateOpts.SetVariables = helpers.TransformAndMergeMap(
		v.GetStringMapString(common.VPkgCreateSet), pkgConfig.CreateOpts.SetVariables, strings.ToUpper)

	return *pkgConfig
}

func WriteGeneratedZarfPackage(pkgConfig *types.PackagerConfig) {
	outputDirectory := pkgConfig.GenerateOpts.Output
	if _, err := os.Stat(outputDirectory); os.IsNotExist(err) {
		outputDirectory = "."
		message.Warn("Directory does not exist: \"" + outputDirectory + "\". Using current directory instead.")
	}

	packageLocation := filepath.Join(outputDirectory, layout.ZarfYAML)
	if _, err := os.Stat(packageLocation); err == nil {
		message.Warn("A " + layout.ZarfYAML + " already exists in the directory: \"" + outputDirectory + "\".")
		packageLocation = filepath.Join(outputDirectory, "zarf-"+pkgConfig.GenerateOpts.Name+".yaml")
	}

	err := utils.WriteYaml(packageLocation, pkgConfig.Pkg, 0644)
	if err != nil {
		message.Fatalf(err, err.Error())
	}

	message.Success(packageLocation + " has been created.")
}
