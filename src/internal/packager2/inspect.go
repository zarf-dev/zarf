// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
)

type ResourceType string

const (
	ManifestResource ResourceType = "manifest"
	ChartResource    ResourceType = "chart"
)

// Resource contains a Kubernetes Manifest or Chart
type Resource struct {
	Content      string
	Name         string
	ResourceType ResourceType
}

type InspectManifestsOptions struct {
	SetVariables map[string]string
	KubeVersion  string
}

type PackageInspectManifestResults struct {
	Resources []Resource
}

// PackageInspectManifests inspects the manifests and charts within each component to find any container images
func PackageInspectManifests(ctx context.Context, pkgLayout *layout2.PackageLayout, opts InspectManifestsOptions) (PackageInspectManifestResults, error) {
	// Set default builtin values
	state, err := types.DefaultZarfState()
	if err != nil {
		return PackageInspectManifestResults{}, err
	}
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opts.SetVariables)
	tmpPackagePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return PackageInspectManifestResults{}, err
	}
	defer os.RemoveAll(tmpPackagePath)

	var resources []Resource
	for _, component := range pkgLayout.Pkg.Components {
		if len(component.Charts)+len(component.Manifests) < 1 {
			// Skip if there are no manifests or charts
			continue
		}
		tmpComponentPath := filepath.Join(tmpPackagePath, component.Name)
		err := os.MkdirAll(tmpComponentPath, helpers.ReadWriteExecuteUser)
		if err != nil {
			return PackageInspectManifestResults{}, err
		}

		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, state)
		if err != nil {
			return PackageInspectManifestResults{}, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		if len(component.Charts) > 0 {
			chartDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ChartsComponentDir)
			if err != nil {
				return PackageInspectManifestResults{}, err
			}
			valuesDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ValuesComponentDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return PackageInspectManifestResults{}, fmt.Errorf("failed to get values: %w", err)
			}

			for _, chart := range component.Charts {
				chartOverrides := make(map[string]any)
				for _, variable := range chart.Variables {
					if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
						// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
						if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
							return PackageInspectManifestResults{}, fmt.Errorf("unable to merge path and value into map: %w", err)
						}
					}
				}
				helmChart, values, err := helm.LoadChartData(chart, chartDir, valuesDir, chartOverrides)
				if err != nil {
					return PackageInspectManifestResults{}, fmt.Errorf("failed to load chart data: %w", err)
				}
				chartTemplate, err := helm.TemplateChart(ctx, chart, helmChart, values, opts.KubeVersion, variableConfig)
				if err != nil {
					return PackageInspectManifestResults{}, fmt.Errorf("could not render the Helm template for chart %s: %w", chart.Name, err)
				}
				resources = append(resources, Resource{
					Content:      chartTemplate,
					Name:         chart.Name,
					ResourceType: ChartResource,
				})
			}
		}

		if len(component.Manifests) > 0 {
			manifestDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ManifestsComponentDir)
			if err != nil {
				return PackageInspectManifestResults{}, fmt.Errorf("failed to get package manifests: %w", err)
			}
			manifestFiles, err := os.ReadDir(manifestDir)
			if err != nil {
				return PackageInspectManifestResults{}, fmt.Errorf("failed to read manifest directory: %w", err)
			}
			for _, file := range manifestFiles {
				path := filepath.Join(manifestDir, file.Name())
				if file.IsDir() {
					continue
				}
				if err := variableConfig.ReplaceTextTemplate(path); err != nil {
					return PackageInspectManifestResults{}, fmt.Errorf("error templating the manifest: %w", err)
				}
				// Read the contents of each file
				contents, err := os.ReadFile(path)
				if err != nil {
					return PackageInspectManifestResults{}, fmt.Errorf("could not read the file %s: %w", path, err)
				}
				resources = append(resources, Resource{
					Content:      string(contents),
					Name:         file.Name(),
					ResourceType: ManifestResource,
				})
			}
		}
	}

	return PackageInspectManifestResults{Resources: resources}, nil
}
