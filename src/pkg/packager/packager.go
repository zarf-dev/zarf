// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains high level operations for Zarf packages
package packager

import (
	"context"
	"os"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
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
	if pkg.GetAPIVersion() != v1alpha1.APIVersion {
		return variableConfig, nil
	}
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
	// Apply any direct overrides specified in the deployment options for this component and chart
	var valuesOverrides map[string]any
	if componentOverrides, ok := opts.valuesOverridesMap[componentName]; ok {
		if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
			valuesOverrides = chartSpecificOverrides
		}
	}

	return helm.GenerateChartOverrides(chart, opts.variableConfig, opts.values, valuesOverrides)
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
		CachePath:                  opts.CachePath,
		RemoteOptions:              opts.RemoteOptions,
		SkipValuesSchemaValidation: opts.SkipValuesSchemaValidation,
	})
	if err != nil {
		return nil, err
	}
	definition := convert.PackageFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: component.Definition.Metadata.Name},
		Values:     component.Definition.Values,
		Components: []v1beta1.Component{{Name: component.Definition.Metadata.Name, ComponentSpec: component.Definition.Component}},
	})
	return &load.ResolvedPackage{
		Definition:   definition,
		Resources:    component.Resources,
		Values:       component.Values,
		ValuesSchema: component.ValuesSchema,
	}, nil
}
