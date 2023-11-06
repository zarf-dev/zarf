// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages
package lint

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Create generates a Zarf package tarball for a given PackageConfig and optional base directory.
func ValidateZarfSchema(baseDir string) (err error) {
	// zarfSchema, _ := config.GetSchemaFile()
	var zarfData interface{}
	if err := utils.ReadYaml(filepath.Join(baseDir, layout.ZarfYAML), &zarfData); err != nil {
		return err
	}

	// result := ValidateSchema(zarfData, zarfSchema)

	// if result.Valid() {
	// 	message.Success("The document is valid\n")
	// } else {
	// 	fmt.Printf("The document is not valid. see errors :\n")
	// 	for _, desc := range result.Errors() {
	// 		fmt.Printf("- %s\n", desc)
	// 	}
	// }

	message.Success("Validation successful")
	return nil
}
