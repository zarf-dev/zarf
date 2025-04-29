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
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
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
	SetVariables map[string]string
	KubeVersion  string
}

type InspectPackageResourcesResults struct {
	Resources []Resource
}

// InspectPackageResources templates and returns the manifests, charts, and values files in the package as they would be on deploy
func InspectPackageResources(ctx context.Context, pkgLayout *layout2.PackageLayout, opts InspectPackageResourcesOptions) (results InspectPackageResourcesResults, err error) {
	s, err := state.Default()
	if err != nil {
		return InspectPackageResourcesResults{}, err
	}
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opts.SetVariables)
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
			chartDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ChartsComponentDir)
			if err != nil {
				return InspectPackageResourcesResults{}, err
			}
			valuesDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ValuesComponentDir)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return InspectPackageResourcesResults{}, fmt.Errorf("failed to get values: %w", err)
			}

			for _, chart := range component.Charts {
				chartOverrides := make(map[string]any)
				for _, variable := range chart.Variables {
					if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
						// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
						if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
							return InspectPackageResourcesResults{}, fmt.Errorf("unable to merge path and value into map: %w", err)
						}
					}
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
			manifestDir, err := pkgLayout.GetComponentDir(tmpComponentPath, component.Name, layout2.ManifestsComponentDir)
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
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkg.Constants)
	variableConfig.PopulateVariables(pkg.Variables, opts.DeploySetVariables)

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

type InspectPackageSbomsOptions struct {
	Architecture            string
	Source                  string
	PublicKeyPath           string
	SkipSignatureValidation bool
	OutputDir               string
}

func InspectPackageSboms(ctx context.Context, opts InspectPackageSbomsOptions) (string, error) {

	// Identify the source type
	srcType, err := identifySource(opts.Source)
	if err != nil {
		return "", err
	}

	// Prepare a temp workspace
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpDir)

	// Note: we do not support inspecting sboms from packages deployed to the cluster
	// as such we won't get anything from the cluster or create a cluster object
	filter := filters.Empty()
	// Fetch the package tar
	isPartial, tarPath, err := fetchPackage(ctx, srcType, opts.Source, "", opts.Architecture, "sbom", tmpDir, filter)
	if err != nil {
		return "", err
	}

	// Load package layout
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  filter,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tarPath, layoutOpt)
	if err != nil {
		return "", err
	}

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()
	outputPath, err := pkgLayout.GetSBOM(opts.OutputDir)
	if err != nil {
		return "", fmt.Errorf("could not get SBOM: %w", err)
	}
	return outputPath, nil

}

// TODO: evaluate if we can de-duplicate these options
type InspectPackageDefinitionOptions struct {
	Architecture            string
	Source                  string
	PublicKeyPath           string
	SkipSignatureValidation bool
}

func InspectPackageDefinition(ctx context.Context, opts InspectPackageDefinitionOptions) (v1alpha1.ZarfPackage, error) {
	// Identify the source type
	srcType, err := identifySource(opts.Source)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}

	// Prepare a temp workspace
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	defer os.Remove(tmpDir)

	// Handle cluster-deployed packages directly
	if srcType == "cluster" {
		cluster, err := cluster.New(ctx) //nolint:errcheck
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}

		pkgLayout, err := loadFromCluster(ctx, opts.Source, cluster)
		if err != nil {
			return v1alpha1.ZarfPackage{}, err
		}
		defer func() {
			err = errors.Join(err, pkgLayout.Cleanup())
		}()
		return pkgLayout.Pkg, nil
	}

	filter := filters.Empty()
	// Fetch the package tar
	isPartial, tarPath, err := fetchPackage(ctx, srcType, opts.Source, "", opts.Architecture, "sbom", tmpDir, filter)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}

	// Load package layout
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  filter,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tarPath, layoutOpt)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	return pkgLayout.Pkg, nil
}

// TODO: evaluate if we can de-duplicate these options
type InspectPackageImagesOptions struct {
	Architecture            string
	Source                  string
	PublicKeyPath           string
	SkipSignatureValidation bool
}

func InspectPackageImages(ctx context.Context, opts InspectPackageImagesOptions) ([]string, error) {

	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	images := getimagesFromPackage(pkgLayout.Pkg)
	if len(images) == 0 {
		return []string{}, fmt.Errorf("no images found in package")
	}

	return images, nil

}

func getimagesFromPackage(pkg v1alpha1.ZarfPackage) []string {
	images := make([]string, 0)
	for _, component := range pkg.Components {
		images = append(images, component.Images...)
	}
	images = helpers.Unique(images)
	return images
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

// loadInspectPackageLayout replicates the Load process while adding inspect targets not utilized in the primary LoadPackage function.
func loadInspectPackageLayout(ctx context.Context, source, architecture, inspectTarget string, cluster *cluster.Cluster, filter filters.ComponentFilterStrategy, publicKeyPath string, skipSignatureValidation bool) (*layout2.PackageLayout, error) {

	// Validate the inspectTarget

	// Identify the source type
	srcType, err := identifySource(source)
	if err != nil {
		return nil, err
	}

	// Prepare a temp workspace
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpDir)

	// Handle cluster-deployed packages directly
	if srcType == "cluster" {
		if cluster != nil {
			return loadFromCluster(ctx, source, cluster)
		}
		return nil, fmt.Errorf("unable to load package from cluster: no cluster connectivity detected")
	}

	// Fetch the package tar
	isPartial, tarPath, err := fetchPackage(ctx, srcType, source, "", architecture, inspectTarget, tmpDir, filter)
	if err != nil {
		return nil, err
	}

	// Load package layout
	layoutOpt := layout.PackageLayoutOptions{
		PublicKeyPath:           publicKeyPath,
		SkipSignatureValidation: skipSignatureValidation,
		IsPartial:               isPartial,
		Filter:                  filter,
	}
	pkgLayout, err := layout.LoadFromTar(ctx, tarPath, layoutOpt)
	if err != nil {
		return nil, err
	}
	return pkgLayout, nil
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
