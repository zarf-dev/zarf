// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager2 is the new implementation for packager.
package packager2

import (
	"context"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func getPopulateVariableConfig(ctx context.Context, pkg v1alpha1.ZarfPackage, setVariables map[string]string) (*variables.VariableConfig, error) {
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkg.Constants)
	variableConfig.PopulateVariables(pkg.Variables, setVariables)
	return variableConfig, nil
}
