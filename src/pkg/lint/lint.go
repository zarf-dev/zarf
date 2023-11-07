// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages
package lint

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Validates a zarf file against the zarf schema, returns an error if the file is invalid
func ValidateZarfSchema(baseDir string) (err error) {
	var typedZarfData types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(baseDir, layout.ZarfYAML), &typedZarfData); err != nil {
		return err
	}

	err = checkForVarInComponentImport(typedZarfData)
	if err != nil {
		message.Warn(err.Error())
	}

	zarfSchema, _ := config.GetSchemaFile()
	var zarfData interface{}
	if err := utils.ReadYaml(filepath.Join(baseDir, layout.ZarfYAML), &zarfData); err != nil {
		return err
	}

	err = validateSchema(zarfData, zarfSchema)

	if err != nil {
		return err
	}

	message.Success("Validation successful")
	return nil
}
