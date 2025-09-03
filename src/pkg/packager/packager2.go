// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains high level operations for Zarf packages
package packager

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

type ValuesOverrides map[string]map[string]map[string]any

// RemoteOptions are common options when calling a remote
type RemoteOptions struct {
	PlainHTTP             bool
	InsecureSkipTLSVerify bool
}

func getPopulatedVariableConfig(ctx context.Context, pkg v1alpha1.ZarfPackage, setVariables map[string]string) (*variables.VariableConfig, error) {
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkg.Constants)
	if err := variableConfig.PopulateVariables(pkg.Variables, setVariables); err != nil {
		return nil, err
	}
	return variableConfig, nil
}

// TODO(mkcp): This can be simplified quite a bit by preprocessing the value overrides with just the chart and values.
func generateValuesOverrides(
	ctx context.Context,
	chart v1alpha1.ZarfChart,
	componentName string,
	variableConfig *variables.VariableConfig,
	valuesOverridesMap ValuesOverrides,
	values value.Values,
) (map[string]any, error) {
	chartOverrides := make(map[string]any)
	valuesOverrides := make(map[string]any)

	for _, variable := range chart.Variables {
		if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
			if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
				return nil, fmt.Errorf("unable to merge path and value into map: %w", err)
			}
		}
	}

	// Map ChartValues' Source to Target
	for _, chartValue := range chart.Values {
		if chartValue.SourcePath == "" || chartValue.TargetPath == "" {
			continue
		}

		// Extract value from source path in values
		sourceValue, err := value.ExtractFromPath(values, value.Path(chartValue.SourcePath))
		if err != nil {
			// Log warning but don't fail - source path might not exist
			logger.From(ctx).Warn("unable to extract value from path",
				"path", chartValue.SourcePath,
				"error", err,
				"component", componentName,
				"chart", chart.Name,
			)
			continue
		}

		// Set value at target path in chart overrides
		if err := helpers.MergePathAndValueIntoMap(chartOverrides, chartValue.TargetPath, sourceValue); err != nil {
			return nil, fmt.Errorf("unable to map value from %s to %s: %w",
				chartValue.SourcePath, chartValue.TargetPath, err)
		}
	}

	// Apply any direct overrides specified in the deployment options for this component and chart
	if componentOverrides, ok := valuesOverridesMap[componentName]; ok {
		if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
			valuesOverrides = chartSpecificOverrides
		}
	}

	return helpers.MergeMapRecursive(chartOverrides, valuesOverrides), nil
}

// OverridePackageNamespace overrides the package namespace if the package contains only one unique namespace
func OverridePackageNamespace(pkg v1alpha1.ZarfPackage, namespace string) error {
	if !pkg.AllowsNamespaceOverride() {
		return fmt.Errorf("cannot override package namespace, metadata.allowNamespaceOverride is false")
	}
	// disallow override on init packages while account for future kinds
	if pkg.Kind != v1alpha1.ZarfPackageConfig {
		return fmt.Errorf("package kind is not a ZarfPackageConfig, cannot override namespace")
	}
	if count := pkg.UniqueNamespaceCount(); count > 1 {
		return fmt.Errorf("package contains %d unique namespaces, cannot override namespace", count)
	}
	pkg.UpdateAllComponentNamespaces(namespace)
	return nil
}
