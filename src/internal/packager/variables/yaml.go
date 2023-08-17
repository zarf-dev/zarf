// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables provides functions for templating yaml files based on Zarf variables and constants.
package variables

import (
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// ProcessYamlFilesInPath iterates over all yaml files in a given path and performs Zarf templating + image swapping.
func (values *Values) ProcessYamlFilesInPath(path string, component types.ZarfComponent) ([]string, error) {
	// Only pull in yml and yaml files
	pattern := regexp.MustCompile(`(?mi)\.ya?ml$`)
	manifests, _ := utils.RecursiveFileList(path, pattern, false)

	for _, manifest := range manifests {
		if err := values.Apply(component, manifest); err != nil {
			return nil, err
		}
	}

	return manifests, nil
}
