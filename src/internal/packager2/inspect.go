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
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
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

type PackageInspectManifestsOptions struct {
	SetVariables map[string]string
	KubeVersion  string
}

type PackageInspectManifestResults struct {
	Resources []Resource
}

// PackageInspectManifests inspects the manifests and charts within each component to find any container images
func PackageInspectManifests(ctx context.Context, pkgLayout *layout2.PackageLayout, opts PackageInspectManifestsOptions) (PackageInspectManifestResults, error) {
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
					Content:      fmt.Sprintf("%s\n", chartTemplate),
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

type DevInspectManifestsOptions struct {
	CreateSetVariables map[string]string
	DeploySetVariables map[string]string
	Flavor             string
	KubeVersion        string
}

type DevInspectManifestResults struct {
	Resources []Resource
}

// DevInspectManifests returns manifests and Helm chart manifests after templating
func DevInspectManifests(ctx context.Context, packagePath string, opts DevInspectManifestsOptions) (DevInspectManifestResults, error) {
	state, err := types.DefaultZarfState()
	if err != nil {
		return DevInspectManifestResults{}, err
	}
	pkg, err := layout.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.CreateSetVariables)
	if err != nil {
		return DevInspectManifestResults{}, err
	}
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkg.Constants)
	variableConfig.PopulateVariables(pkg.Variables, opts.DeploySetVariables)

	tmpPackagePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return DevInspectManifestResults{}, err
	}
	defer os.RemoveAll(tmpPackagePath)

	var resources []Resource
	for _, component := range pkg.Components {
		if len(component.Charts)+len(component.Manifests) < 1 {
			continue
		}
		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, state)
		if err != nil {
			return DevInspectManifestResults{}, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		compBuildPath := filepath.Join(tmpPackagePath, component.Name)
		err = os.MkdirAll(compBuildPath, 0o700)
		if err != nil {
			return DevInspectManifestResults{}, err
		}

		for _, zarfChart := range component.Charts {
			chartTemplate, err := templateChart(ctx, zarfChart, packagePath, compBuildPath, variableConfig, opts.KubeVersion)
			if err != nil {
				return DevInspectManifestResults{}, err
			}
			resources = append(resources, Resource{
				Content:      fmt.Sprintf("%s\n", chartTemplate),
				Name:         zarfChart.Name,
				ResourceType: ChartResource,
			})
		}

		manifestDir := filepath.Join(compBuildPath, string(layout.ManifestsComponentDir))
		if len(component.Manifests) > 0 {
			err := os.MkdirAll(manifestDir, 0o700)
			if err != nil {
				return DevInspectManifestResults{}, err
			}
		}
		for _, manifest := range component.Manifests {
			manifestPaths := []string{}
			for idx, path := range manifest.Kustomizations {
				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
				rel := filepath.Join(string(layout.ManifestsComponentDir), kname)
				dst := filepath.Join(compBuildPath, rel)
				if !helpers.IsURL(path) {
					path = filepath.Join(packagePath, path)
				}
				// Generate manifests from kustomizations and place in the package
				if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
					return DevInspectManifestResults{}, fmt.Errorf("unable to build the kustomization for %s: %w", path, err)
				}
				manifestPaths = append(manifestPaths, dst)
			}
			// Get all manifest files
			for idx, f := range manifest.Files {
				rel := filepath.Join(string(layout.ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, idx))
				dst := filepath.Join(compBuildPath, rel)
				if helpers.IsURL(f) {
					if err := utils.DownloadToFile(ctx, f, dst, component.DeprecatedCosignKeyPath); err != nil {
						return DevInspectManifestResults{}, fmt.Errorf(lang.ErrDownloading, f, err.Error())
					}
				} else {
					if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, f), dst); err != nil {
						return DevInspectManifestResults{}, fmt.Errorf("unable to copy manifest %s: %w", f, err)
					}
				}
				manifestPaths = append(manifestPaths, dst)
			}

			for _, f := range manifestPaths {
				if err := variableConfig.ReplaceTextTemplate(f); err != nil {
					return DevInspectManifestResults{}, err
				}
				contents, err := os.ReadFile(f)
				if err != nil {
					return DevInspectManifestResults{}, err
				}
				resources = append(resources, Resource{
					Content:      string(contents),
					Name:         f,
					ResourceType: ManifestResource,
				})
			}
		}
	}

	return DevInspectManifestResults{Resources: resources}, nil
}

// templateChart returns a templated chart.yaml as a string after templating
func templateChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, packagePath string, compBuildPath string, variableConfig *variables.VariableConfig, kubeVersion string) (string, error) {
	if zarfChart.LocalPath != "" {
		zarfChart.LocalPath = filepath.Join(packagePath, zarfChart.LocalPath)
	}
	valuesFiles := []string{}
	oldValuesFiles := zarfChart.ValuesFiles
	for _, v := range zarfChart.ValuesFiles {
		valuesFiles = append(valuesFiles, filepath.Join(packagePath, v))
	}
	zarfChart.ValuesFiles = valuesFiles
	chartPath := filepath.Join(compBuildPath, string(layout.ChartsComponentDir))
	valuesFilePath := filepath.Join(compBuildPath, string(layout.ValuesComponentDir))
	if err := helm.PackageChart(ctx, zarfChart, chartPath, valuesFilePath); err != nil {
		return "", fmt.Errorf("unable to package the chart %s: %w", zarfChart.Name, err)
	}
	zarfChart.ValuesFiles = oldValuesFiles

	chartOverrides := make(map[string]any)
	for _, variable := range zarfChart.Variables {
		if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
			if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
				return "", fmt.Errorf("unable to merge path and value into map: %w", err)
			}
		}
	}

	valuesFilePaths, err := helpers.RecursiveFileList(valuesFilePath, nil, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("failed to list values files: %w", err)
	}
	for _, valueFilePath := range valuesFilePaths {
		err := variableConfig.ReplaceTextTemplate(valueFilePath)
		if err != nil {
			return "", fmt.Errorf("error templating the values file: %w", err)
		}
	}

	chart, values, err := helm.LoadChartData(zarfChart, chartPath, valuesFilePath, chartOverrides)
	if err != nil {
		return "", fmt.Errorf("failed to load chart data: %w", err)
	}
	chartTemplate, err := helm.TemplateChart(ctx, zarfChart, chart, values, kubeVersion, variableConfig)
	if err != nil {
		return "", fmt.Errorf("could not render the Helm template for chart %s: %w", zarfChart.Name, err)
	}
	return chartTemplate, nil
}
