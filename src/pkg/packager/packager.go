// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains high level operations for Zarf packages
package packager

import (
	"context"
	"fmt"
	"os"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// ValuesOverrides is a map of component names to chart names containing Helm Chart values to override values on deploy.
type ValuesOverrides map[string]map[string]map[string]any

func getPopulatedVariableConfig(ctx context.Context, pkg api.Package, setVariables map[string]string, isInteractive bool) (*variables.VariableConfig, error) {
	variableConfig := template.GetZarfVariableConfig(ctx, isInteractive)
	variableConfig.SetConstants(pkg.Constants)
	if err := variableConfig.PopulateVariables(pkg.Variables, setVariables); err != nil {
		return nil, err
	}
	return variableConfig, nil
}

type overrideOpts struct {
	variableConfig     *variables.VariableConfig
	values             value.Values
	valuesOverridesMap ValuesOverrides
}

// generateValuesOverrides generates a map of values to override for a given chart and component, with precedence of:
// Zarf Variable overrides -> Zarf value overrides -> direct API helm-value overrides.
func generateValuesOverrides(_ context.Context, chart api.Chart, componentName string, opts overrideOpts) (map[string]any, error) {
	chartOverrides := make(value.Values)
	valuesOverrides := make(map[string]any)

	for _, variable := range chart.Variables {
		if setVar, ok := opts.variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Add leading dot to variable.Path to create a valid value.Path
			path := "." + variable.Path
			if err := chartOverrides.Set(value.Path(path), setVar.Value); err != nil {
				return nil, fmt.Errorf("unable to set value at path %s: %w", path, err)
			}
		}
	}

	// Map ChartValues' Source to Target
	for _, chartValue := range chart.Values {
		if chartValue.SourcePath == "" || chartValue.TargetPath == "" {
			return nil, fmt.Errorf("sourcePath \"%s\" and targetPath \"%s\" must not be empty", chartValue.SourcePath, chartValue.TargetPath)
		}
		if chartValue.SourcePath[0] != '.' {
			return nil, fmt.Errorf("sourcePath \"%s\" must start with a dot", chartValue.SourcePath)
		}
		if chartValue.TargetPath[0] != '.' {
			return nil, fmt.Errorf("targetPath \"%s\" must start with a dot", chartValue.TargetPath)
		}

		// Drop any excluded sub-paths before extracting. Work on a copy so the
		// shared source values are not mutated for other charts.
		sourceValues := opts.values
		if len(chartValue.ExcludePaths) > 0 {
			sourceValues = opts.values.DeepCopy()
			for _, excludePath := range chartValue.ExcludePaths {
				if err := sourceValues.Delete(value.Path(excludePath)); err != nil {
					return nil, fmt.Errorf("unable to exclude path %s: %w", excludePath, err)
				}
			}
		}

		// Extract value from source path in values
		sourceValue, err := sourceValues.Extract(value.Path(chartValue.SourcePath))
		if err != nil {
			return nil, fmt.Errorf("unable to extract value source: %w", err)
		}

		// Set value at targetPath in chart overrides
		if err := chartOverrides.Set(value.Path(chartValue.TargetPath), sourceValue); err != nil {
			return nil, fmt.Errorf("unable to map value from %s to %s: %w",
				chartValue.SourcePath, chartValue.TargetPath, err)
		}
	}

	// Apply any direct overrides specified in the deployment options for this component and chart
	if componentOverrides, ok := opts.valuesOverridesMap[componentName]; ok {
		if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
			valuesOverrides = chartSpecificOverrides
		}
	}

	// Merge valuesOverrides into chartOverrides (valuesOverrides takes precedence)
	chartOverrides.DeepMerge(valuesOverrides)
	return chartOverrides, nil
}

func loadDefinition(ctx context.Context, source string, opts load.PackageOptions) (*load.ResolvedPackage, error) {
	resolvedPath, err := layout.ResolvePackagePath(source)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(resolvedPath.ManifestFile)
	if err != nil {
		return nil, err
	}
	header, err := load.ParseDefinitionHeader(contents)
	if err != nil {
		return nil, err
	}
	if header.Kind != string(v1beta1.ZarfComponentConfig) {
		return load.Package(ctx, source, opts)
	}
	component, err := load.Component(ctx, resolvedPath.ManifestFile, load.ComponentOptions{
		CachePath:     opts.CachePath,
		RemoteOptions: opts.RemoteOptions,
	})
	if err != nil {
		return nil, err
	}
	definition := convert.PackageFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: component.Definition.Metadata.Name},
		Components: []v1beta1.Component{{Name: component.Definition.Metadata.Name, ComponentSpec: component.Definition.Component}},
	})
	return &load.ResolvedPackage{Definition: definition, Resources: component.Resources, Values: component.Values}, nil
}
