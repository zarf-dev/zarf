// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// UpdatePackageConfigForGenerate updates the package configuration with the new ZarfPackageConfig from pkgConfig.GenerateOpts
func UpdatePackageConfigForGenerate(pkgConfig *types.PackagerConfig) types.PackagerConfig {

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

	pkgConfig.Pkg.Kind = types.ZarfPackageConfig
	pkgConfig.Pkg.Metadata.Name = pkgConfig.GenerateOpts.Name
	pkgConfig.Pkg.Components = append(newComponent, pkgConfig.Pkg.Components...)

	// Set config for FindImages and helm
	pkgConfig.CreateOpts.BaseDir = "."
	pkgConfig.FindImagesOpts.RepoHelmChartPath = pkgConfig.GenerateOpts.GitPath

	return *pkgConfig
}

// WriteGeneratedZarfPackage writes the generated ZarfPackageConfig to the output directory and filename avoiding overwriting existing files
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
