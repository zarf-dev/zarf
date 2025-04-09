// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
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

type InspectManifestsOptions struct {
	SetVariables map[string]string
	KubeVersion  string
}

// PackageInspectManifests inspects the manifests and charts within each component to find any container images
func PackageInspectManifests(ctx context.Context, pkgLayout *layout2.PackageLayout, opts InspectManifestsOptions) ([]Resource, error) {
	// Set default builtin values
	registryInfo := types.RegistryInfo{}
	if err := registryInfo.FillInEmptyValues(); err != nil {
		return nil, err
	}
	gitServer := types.GitServerInfo{}
	if err := gitServer.FillInEmptyValues(); err != nil {
		return nil, err
	}
	artifactServer := types.ArtifactServerInfo{}
	artifactServer.FillInEmptyValues()
	state := &types.ZarfState{
		RegistryInfo:   registryInfo,
		GitServer:      gitServer,
		ArtifactServer: artifactServer,
	}
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opts.SetVariables)
	tmpPackagePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
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
			return nil, err
		}

		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, state)
		if err != nil {
			return nil, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		if len(component.Charts) > 0 {
			chartDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ChartsComponentDir)
			if err != nil {
				return nil, err
			}
			valuesDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ValuesComponentDir)
			if err != nil {
				return nil, err
			}

			for _, chart := range component.Charts {
				//FIX ME values overrides
				helmChart, values, err := helm.LoadChartData(chart, chartDir, valuesDir, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to load chart data: %w", err)
				}
				chartTemplate, err := helm.TemplateChart(ctx, chart, helmChart, values, opts.KubeVersion, variableConfig)
				if err != nil {
					return nil, fmt.Errorf("could not render the Helm template for chart %s: %w", chart.Name, err)
				}
				// FIXME add comment at the top of the chart
				resources = append(resources, Resource{
					Content:      chartTemplate,
					Name:         chart.Name,
					ResourceType: "chart",
				})
			}
		}

		if len(component.Manifests) > 0 {
			manifestDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ManifestsComponentDir)
			if err != nil {
				return nil, fmt.Errorf("failed to get package manifests: %w", err)
			}
			manifestFiles, err := os.ReadDir(manifestDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read manifest directory: %w", err)
			}
			for _, file := range manifestFiles {
				path := filepath.Join(manifestDir, file.Name())
				if file.IsDir() {
					continue
				}
				if err := variableConfig.ReplaceTextTemplate(path); err != nil {
					return nil, fmt.Errorf("error templating the manifest: %w", err)
				}
				// Read the contents of each file
				contents, err := os.ReadFile(path)
				if err != nil {
					return nil, fmt.Errorf("could not read the file %s: %w", path, err)
				}
				resources = append(resources, Resource{
					Content:      string(contents),
					Name:         file.Name(),
					ResourceType: "manifest",
				})
			}
		}
	}

	return resources, nil
}
