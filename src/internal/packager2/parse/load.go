// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package parse parses zarf.yaml files into Zarf objects
package parse

import (
	"context"

	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// ZarfPackage parses the yaml passed as a byte slice and applies potential schema migrations.
func ZarfPackage(ctx context.Context, b []byte) (v1alpha1.ZarfPackage, error) {
	var pkg v1alpha1.ZarfPackage
	err := goyaml.Unmarshal(b, &pkg)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	pkg, warnings := migrateDeprecated(pkg)
	for _, warning := range warnings {
		logger.From(ctx).Warn(warning)
	}
	return pkg, nil
}
