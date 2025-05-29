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
	"github.com/zarf-dev/zarf/src/internal/packager2/filters"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"helm.sh/helm/v3/pkg/chartutil"
)

type ResourceType string

const (
	ManifestResource   ResourceType = "manifest"
	ChartResource      ResourceType = "chart"
	ValuesFileResource ResourceType = "valuesfile"
)

// Resource contains a Kubernetes Manifest or Chart
type Resource struct {
	Content      string
	Name         string
	ResourceType ResourceType
}

type InspectPackageResourcesOptions struct {
	Architecture            string
	Components              string
	PublicKeyPath           string
	SkipSignatureValidation bool
	SetVariables            map[string]string
	KubeVersion             string
}

type InspectPackageResourcesResults struct {
	Resources []Resource
}

// InspectPackageResources templates and returns the manifests, charts, and values files in the package as they would be on deploy
func InspectPackageResources(ctx context.Context, source string, opts InspectPackageResourcesOptions) (results InspectPackageResourcesResults, err error) {
	s, err := state.Default()
	if err != nil {
		return InspectPackageResourcesResults{}, err
	}

	loadOpts := LoadOptions{
		Source:                  source,
		Architecture:            opts.Architecture,
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		LayersSelector:          zoci.ComponentLayers,
		Filter:                  filters.BySelectState(opts.Components),
	}

	pkgLayout, err := LoadPackage(ctx, loadOpts)
	if err != nil {
		return InspectPackageResourcesResults{}, err
	}

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	variableConfig, err := getPopulatedVariableConfig(ctx, pkgLayout.Pkg, opts.SetVariables)
	if err != nil {
		return InspectPackageResourcesResults{}, err
	}
	tmpPackagePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return InspectPackageResourcesResults{}, err
	}
	defer func(path string) {
		errRemove := os.RemoveAll(path)
		err = errors.Join(err, errRemove)
	}(tmpPackagePath)

	var resources []Resource
	for _, component := range pkgLayout.Pkg.Components {
		tmpComponentPath := filepath.Join(tmpPackagePath, component.Name)
		err := os.MkdirAll(tmpComponentPath, helpers.ReadWriteExecuteUser)
		if err != nil {
			return InspectPackageResourcesResults{}, err
		}

		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, s)
		if err != nil {
			return InspectPackageResourcesResults{}, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		if len(component.Charts) > 0 {
			chartDir, err := pkgLayout.GetComponentDir(ctx, tmpComponentPath, component.Name, layout.ChartsComponentDir)
			if err != nil {
				return InspectPackageResourcesResults{}, err
			}
			valuesDir, err := pkgLayout.GetComponentDir(ctx, tmpComponentPath, component.Name, layout.ValuesComponentDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return InspectPackageResourcesResults{}, fmt.Errorf("failed to get values: %w", err)
			}

			for _, chart := range component.Charts {
				chartOverrides, err := generateValuesOverrides(chart, component.Name, variableConfig, nil)
				if err != nil {
					return InspectPackageResourcesResults{}, err
				}
				if err := templateValuesFiles(chart, valuesDir, variableConfig); err != nil {
					return InspectPackageResourcesResults{}, err
				}

				helmChart, values, err := helm.LoadChartData(chart, chartDir, valuesDir, chartOverrides)
				if err != nil {
					return InspectPackageResourcesResults{}, fmt.Errorf("failed to load chart data: %w", err)
				}
				chartTemplate, err := helm.TemplateChart(ctx, chart, helmChart, values, opts.KubeVersion, variableConfig)
				if err != nil {
					return InspectPackageResourcesResults{}, fmt.Errorf("could not render the Helm template for chart %s: %w", chart.Name, err)
				}
				resources = append(resources, Resource{
					Content:      fmt.Sprintf("%s\n", chartTemplate),
					Name:         chart.Name,
					ResourceType: ChartResource,
				})
				valuesYaml, err := values.YAML()
				if err != nil {
					return InspectPackageResourcesResults{}, fmt.Errorf("failed to get values: %w", err)
				}
				resources = append(resources, Resource{
					Content:      fmt.Sprintf("%s", valuesYaml),
					Name:         chart.Name,
					ResourceType: ValuesFileResource,
				})
			}
		}

		if len(component.Manifests) > 0 {
			manifestDir, err := pkgLayout.GetComponentDir(ctx, tmpComponentPath, component.Name, layout.ManifestsComponentDir)
			if err != nil {
				return InspectPackageResourcesResults{}, fmt.Errorf("failed to get package manifests: %w", err)
			}
			manifestFiles, err := os.ReadDir(manifestDir)
			if err != nil {
				return InspectPackageResourcesResults{}, fmt.Errorf("failed to read manifest directory: %w", err)
			}
			for _, file := range manifestFiles {
				path := filepath.Join(manifestDir, file.Name())
				if file.IsDir() {
					continue
				}
				if err := variableConfig.ReplaceTextTemplate(path); err != nil {
					return InspectPackageResourcesResults{}, fmt.Errorf("error templating the manifest: %w", err)
				}
				contents, err := os.ReadFile(path)
				if err != nil {
					return InspectPackageResourcesResults{}, fmt.Errorf("could not read the file %s: %w", path, err)
				}
				resources = append(resources, Resource{
					Content:      string(contents),
					Name:         file.Name(),
					ResourceType: ManifestResource,
				})
			}
		}
	}

	return InspectPackageResourcesResults{Resources: resources}, nil
}

func templateValuesFiles(chart v1alpha1.ZarfChart, valuesDir string, variableConfig *variables.VariableConfig) error {
	for idx := range chart.ValuesFiles {
		valueFilePath := helm.StandardValuesName(valuesDir, chart, idx)
		if err := variableConfig.ReplaceTextTemplate(valueFilePath); err != nil {
			return fmt.Errorf("error templating values file %s: %w", valueFilePath, err)
		}
	}
	return nil
}

type InspectDefinitionResourcesOptions struct {
	CreateSetVariables map[string]string
	DeploySetVariables map[string]string
	Flavor             string
	KubeVersion        string
}

type InspectDefinitionResourcesResults struct {
	Resources []Resource
}

// InspectDefinitionResources templates and returns the manifests and Helm chart manifests found in the zarf.yaml at the given path
func InspectDefinitionResources(ctx context.Context, packagePath string, opts InspectDefinitionResourcesOptions) (results InspectDefinitionResourcesResults, err error) {
	s, err := state.Default()
	if err != nil {
		return InspectDefinitionResourcesResults{}, err
	}
	pkg, err := layout.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.CreateSetVariables)
	if err != nil {
		return InspectDefinitionResourcesResults{}, err
	}
	variableConfig, err := getPopulatedVariableConfig(ctx, pkg, opts.DeploySetVariables)
	if err != nil {
		return InspectDefinitionResourcesResults{}, err
	}

	tmpPackagePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return InspectDefinitionResourcesResults{}, err
	}
	defer func(path string) {
		errRemove := os.RemoveAll(path)
		err = errors.Join(err, errRemove)
	}(tmpPackagePath)

	var resources []Resource
	for _, component := range pkg.Components {
		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, s)
		if err != nil {
			return InspectDefinitionResourcesResults{}, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		compBuildPath := filepath.Join(tmpPackagePath, component.Name)
		err = os.MkdirAll(compBuildPath, 0o700)
		if err != nil {
			return InspectDefinitionResourcesResults{}, err
		}

		for _, zarfChart := range component.Charts {
			chartResource, values, err := getTemplatedChart(ctx, zarfChart, packagePath, compBuildPath, variableConfig, opts.KubeVersion)
			if err != nil {
				return InspectDefinitionResourcesResults{}, err
			}
			resources = append(resources, chartResource)
			valuesYaml, err := values.YAML()
			if err != nil {
				return InspectDefinitionResourcesResults{}, err
			}
			resources = append(resources, Resource{
				Content:      fmt.Sprintf("%s", valuesYaml),
				Name:         zarfChart.Name,
				ResourceType: ValuesFileResource,
			})
		}

		manifestDir := filepath.Join(compBuildPath, string(layout.ManifestsComponentDir))
		if len(component.Manifests) > 0 {
			err := os.MkdirAll(manifestDir, 0o700)
			if err != nil {
				return InspectDefinitionResourcesResults{}, err
			}
		}
		for _, manifest := range component.Manifests {
			manifestResources, err := getTemplatedManifests(ctx, manifest, packagePath, compBuildPath, variableConfig)
			if err != nil {
				return InspectDefinitionResourcesResults{}, err
			}
			resources = append(resources, manifestResources...)
		}
	}

	return InspectDefinitionResourcesResults{Resources: resources}, nil
}

type InspectPackageSbomsResult struct {
	Path string
}

type InspectPackageSbomsOptions struct {
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
	OutputDir               string
}

func InspectPackageSboms(ctx context.Context, source string, opts InspectPackageSbomsOptions) (InspectPackageSbomsResult, error) {

	loadOpts := LoadOptions{
		Source:                  source,
		Architecture:            opts.Architecture,
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		LayersSelector:          zoci.SbomLayers,
		Filter:                  filters.Empty(),
	}
	pkgLayout, err := LoadPackage(ctx, loadOpts)
	if err != nil {
		return InspectPackageSbomsResult{}, fmt.Errorf("unable to load the package: %w", err)
	}

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()
	outputPath := filepath.Join(opts.OutputDir, pkgLayout.Pkg.Metadata.Name)
	err = pkgLayout.GetSBOM(ctx, outputPath)
	if err != nil {
		return InspectPackageSbomsResult{}, fmt.Errorf("could not get SBOM: %w", err)
	}
	return InspectPackageSbomsResult{
		Path: outputPath,
	}, nil

}

type InspectPackageDefinitionResult struct {
	Package v1alpha1.ZarfPackage
}

type InspectPackageDefinitionOptions struct {
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
}

func InspectPackageDefinition(ctx context.Context, source string, opts InspectPackageDefinitionOptions) (InspectPackageDefinitionResult, error) {
	cluster, _ := cluster.New(ctx) //nolint:errcheck

	pkg, err := GetPackageFromSourceOrCluster(ctx, cluster, source, opts.SkipSignatureValidation, opts.PublicKeyPath, zoci.MetadataLayers)
	if err != nil {
		return InspectPackageDefinitionResult{}, fmt.Errorf("unable to load the package: %w", err)
	}

	return InspectPackageDefinitionResult{
		Package: pkg,
	}, nil
}

// Each result contains an image. This allows expansion to other metadata
type InspectPackageImageResult struct {
	Images []string
}

type InspectPackageImagesOptions struct {
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
}

func InspectPackageImages(ctx context.Context, source string, opts InspectPackageImagesOptions) (InspectPackageImageResult, error) {

	cluster, _ := cluster.New(ctx) //nolint:errcheck

	pkg, err := GetPackageFromSourceOrCluster(ctx, cluster, source, opts.SkipSignatureValidation, opts.PublicKeyPath, zoci.MetadataLayers)
	if err != nil {
		return InspectPackageImageResult{}, fmt.Errorf("unable to load the package: %w", err)
	}

	images := make([]string, 0)
	for _, component := range pkg.Components {
		images = append(images, component.Images...)
	}
	images = helpers.Unique(images)
	if len(images) == 0 {
		return InspectPackageImageResult{}, fmt.Errorf("no images found in package")
	}

	return InspectPackageImageResult{
		Images: images,
	}, nil

}

func getTemplatedManifests(ctx context.Context, manifest v1alpha1.ZarfManifest, packagePath string, baseComponentDir string, variableConfig *variables.VariableConfig) ([]Resource, error) {
	manifestPaths := []string{}
	for idx, path := range manifest.Kustomizations {
		kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
		rel := filepath.Join(string(layout.ManifestsComponentDir), kname)
		dst := filepath.Join(baseComponentDir, rel)
		if !helpers.IsURL(path) {
			path = filepath.Join(packagePath, path)
		}
		// Generate manifests from kustomizations and place in the package
		if err := kustomize.Build(path, dst, manifest.KustomizeAllowAnyDirectory); err != nil {
			return nil, fmt.Errorf("unable to build the kustomization for %s: %w", path, err)
		}
		manifestPaths = append(manifestPaths, dst)
	}
	// Get all manifest files
	for idx, f := range manifest.Files {
		rel := filepath.Join(string(layout.ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, idx))
		dst := filepath.Join(baseComponentDir, rel)
		if helpers.IsURL(f) {
			if err := utils.DownloadToFile(ctx, f, dst, ""); err != nil {
				return nil, fmt.Errorf(lang.ErrDownloading, f, err.Error())
			}
		} else {
			if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, f), dst); err != nil {
				return nil, fmt.Errorf("unable to copy manifest %s: %w", f, err)
			}
		}
		manifestPaths = append(manifestPaths, dst)
	}
	var resources []Resource
	for _, manifest := range manifestPaths {
		if err := variableConfig.ReplaceTextTemplate(manifest); err != nil {
			return nil, fmt.Errorf("error templating the manifest: %w", err)
		}
		content, err := os.ReadFile(manifest)
		if err != nil {
			return nil, err
		}
		resources = append(resources, Resource{
			Content:      string(content),
			Name:         manifest,
			ResourceType: ManifestResource,
		})
	}
	return resources, nil
}

// getTemplatedChart returns a templated chart.yaml as a string after templating
func getTemplatedChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, packagePath string, baseComponentDir string, variableConfig *variables.VariableConfig, kubeVersion string) (Resource, chartutil.Values, error) {
	if zarfChart.LocalPath != "" {
		zarfChart.LocalPath = filepath.Join(packagePath, zarfChart.LocalPath)
	}
	valuesFiles := []string{}
	oldValuesFiles := zarfChart.ValuesFiles
	for _, v := range zarfChart.ValuesFiles {
		valuesFiles = append(valuesFiles, filepath.Join(packagePath, v))
	}
	zarfChart.ValuesFiles = valuesFiles
	chartPath := filepath.Join(baseComponentDir, string(layout.ChartsComponentDir))
	valuesFilePath := filepath.Join(baseComponentDir, string(layout.ValuesComponentDir))
	if err := helm.PackageChart(ctx, zarfChart, chartPath, valuesFilePath); err != nil {
		return Resource{}, chartutil.Values{}, fmt.Errorf("unable to package the chart %s: %w", zarfChart.Name, err)
	}
	zarfChart.ValuesFiles = oldValuesFiles

	chartOverrides := make(map[string]any)
	for _, variable := range zarfChart.Variables {
		if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
			if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
				return Resource{}, chartutil.Values{}, fmt.Errorf("unable to merge path and value into map: %w", err)
			}
		}
	}

	valuesFilePaths, err := helpers.RecursiveFileList(valuesFilePath, nil, false)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Resource{}, chartutil.Values{}, fmt.Errorf("failed to list values files: %w", err)
	}
	for _, valueFilePath := range valuesFilePaths {
		err := variableConfig.ReplaceTextTemplate(valueFilePath)
		if err != nil {
			return Resource{}, chartutil.Values{}, fmt.Errorf("error templating the values file: %w", err)
		}
	}

	chart, values, err := helm.LoadChartData(zarfChart, chartPath, valuesFilePath, chartOverrides)
	if err != nil {
		return Resource{}, chartutil.Values{}, fmt.Errorf("failed to load chart data: %w", err)
	}
	chartTemplate, err := helm.TemplateChart(ctx, zarfChart, chart, values, kubeVersion, variableConfig)
	if err != nil {
		return Resource{}, chartutil.Values{}, fmt.Errorf("could not render the Helm template for chart %s: %w", zarfChart.Name, err)
	}
	resource := Resource{
		Content:      fmt.Sprintf("%s\n", chartTemplate),
		Name:         zarfChart.Name,
		ResourceType: ChartResource,
	}
	return resource, values, nil
}
