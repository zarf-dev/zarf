// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/distribution/reference"
	"github.com/goccy/go-yaml"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	imageCheck      = regexp.MustCompile(`(?mi)"image":"((([a-z0-9._-]+)/)?([a-z0-9._-]+)(:([a-z0-9._-]+))?)"`)
	imageFuzzyCheck = regexp.MustCompile(`(?mi)["|=]([a-z0-9\-.\/:]+:[\w.\-]*[a-z\.\-][\w.\-]*)"`)
)

// FindImagesOptions declares the parameters to find images.
type FindImagesOptions struct {
	// RepoHelmChartPath specifies the path to helm charts in git repos defined in the zarf.yaml
	RepoHelmChartPath string
	// RegistryURL specifies the value of the ###ZARF_REGISTRY### variable during templating
	RegistryURL string
	// KubeVersionOverride specifies the kubernetes version to provide the Helm chart
	KubeVersionOverride string
	// CreateSetVariables specifies the package create templates
	CreateSetVariables map[string]string
	// DeploySetVariables specifies the package deploy variables
	DeploySetVariables map[string]string
	// Flavor specifies the flavor to use
	Flavor string
	// Why specifies the image to look for so we can print the containing manifest
	Why string
	// SkipCosign specifies whether to skip cosign artifact lookups
	SkipCosign bool
}

// FindImagesResult contains the results of FindImages for a package
type FindImagesResult struct {
	ComponentImageScans []ComponentImageScan
}

// ComponentImageScan contains the results of FindImages for a component
type ComponentImageScan struct {
	// ComponentName is the name of the component where the images were found
	ComponentName string
	// Matches contains definitively identified container images, such as those in an image: field
	Matches []string
	// PotentialMatches contains potential container images found by a regex
	PotentialMatches []string
	// CosignArtifacts contains found cosign artifacts for images
	CosignArtifacts []string
	// WhyResources contains the resources where specific images were found (when Why option is used)
	WhyResources []Resource
}

// FindImages iterates over the manifests and charts within each component to find any container images
// It returns a FindImageResults which contains a scan result for each component
func FindImages(ctx context.Context, packagePath string, opts FindImagesOptions) (FindImagesResult, error) {
	l := logger.From(ctx)
	pkg, err := layout.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.CreateSetVariables)
	if err != nil {
		return FindImagesResult{}, err
	}

	state, err := types.DefaultZarfState()
	if err != nil {
		return FindImagesResult{}, err
	}
	state.RegistryInfo.Address = opts.RegistryURL
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkg.Constants)
	variableConfig.PopulateVariables(pkg.Variables, opts.DeploySetVariables)
	tmpBuildPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return FindImagesResult{}, err
	}
	defer os.RemoveAll(tmpBuildPath)

	componentImageScans := []ComponentImageScan{}
	for _, component := range pkg.Components {
		if len(component.Charts)+len(component.Manifests)+len(component.Repos) < 1 {
			// Skip if there are no manifests, charts, or repos
			continue
		}
		scan := ComponentImageScan{ComponentName: component.Name}

		applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, state)
		if err != nil {
			return FindImagesResult{}, err
		}
		variableConfig.SetApplicationTemplates(applicationTemplates)

		compBuildPath := filepath.Join(tmpBuildPath, component.Name)
		err = os.MkdirAll(compBuildPath, 0o700)
		if err != nil {
			return FindImagesResult{}, err
		}

		if opts.RepoHelmChartPath != "" {
			// Also process git repos that have helm charts
			for idx, repo := range component.Repos {
				matches := strings.Split(repo, "@")
				if len(matches) < 2 {
					return FindImagesResult{}, fmt.Errorf("cannot convert the Git repository %s to a Helm chart without a version tag", repo)
				}
				// If a repo helm chart path is specified,
				component.Charts = append(component.Charts, v1alpha1.ZarfChart{
					Name:    fmt.Sprintf("temp-git-chart-%d", idx),
					URL:     matches[0],
					Version: matches[1],
					// Trim the first char to match how the packager expects it, this is messy,need to clean up better
					GitPath: strings.TrimPrefix(opts.RepoHelmChartPath, "/"),
				})
			}
		}

		resources := []*unstructured.Unstructured{}
		matchedImages := map[string]bool{}
		maybeImages := map[string]bool{}
		for _, zarfChart := range component.Charts {
			// Generate helm templates for this chart
			if zarfChart.LocalPath != "" {
				zarfChart.LocalPath = filepath.Join(packagePath, zarfChart.LocalPath)
			}
			oldValuesFiles := zarfChart.ValuesFiles
			valuesFiles := []string{}
			for _, v := range zarfChart.ValuesFiles {
				valuesFiles = append(valuesFiles, filepath.Join(packagePath, v))
			}
			zarfChart.ValuesFiles = valuesFiles
			chartPath := filepath.Join(compBuildPath, string(layout.ChartsComponentDir))
			valuesFilePath := filepath.Join(compBuildPath, string(layout.ValuesComponentDir))
			if err := helm.PackageChart(ctx, zarfChart, chartPath, valuesFilePath); err != nil {
				return FindImagesResult{}, fmt.Errorf("unable to package the chart %s: %w", zarfChart.Name, err)
			}
			zarfChart.ValuesFiles = oldValuesFiles

			valuesFilePaths, err := helpers.RecursiveFileList(valuesFilePath, nil, false)
			// TODO: The values path should exist if the path is set, otherwise it should be empty.
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return FindImagesResult{}, err
			}
			for _, valueFilePath := range valuesFilePaths {
				err := variableConfig.ReplaceTextTemplate(valueFilePath)
				if err != nil {
					return FindImagesResult{}, err
				}
			}

			chart, values, err := helm.LoadChartData(zarfChart, chartPath, valuesFilePath, nil)
			if err != nil {
				return FindImagesResult{}, fmt.Errorf("failed to load chart data: %w", err)
			}
			chartTemplate, err := helm.TemplateChart(ctx, zarfChart, chart, values, opts.KubeVersionOverride, variableConfig)
			if err != nil {
				return FindImagesResult{}, fmt.Errorf("could not render the Helm template for chart %s: %w", zarfChart.Name, err)
			}

			// Break the template into separate resources
			yamls, err := utils.SplitYAML([]byte(chartTemplate))
			if err != nil {
				return FindImagesResult{}, err
			}
			resources = append(resources, yamls...)

			chartTarball := helm.StandardName(chartPath, zarfChart) + ".tgz"
			annotatedImages, err := helm.FindAnnotatedImagesForChart(chartTarball, values)
			if err != nil {
				return FindImagesResult{}, fmt.Errorf("could not look up image annotations for chart URL %s: %w", zarfChart.URL, err)
			}
			for _, image := range annotatedImages {
				matchedImages[image] = true
			}

			// Check if the --why flag is set
			if opts.Why != "" {
				var err error
				whyResources, err := findWhyResources(yamls, opts.Why, zarfChart.Name)
				if err != nil {
					return FindImagesResult{}, fmt.Errorf("could not determine why resource for the chart %s: %w", zarfChart.Name, err)
				}
				for _, w := range whyResources {
					w.ResourceType = ChartResource
					scan.WhyResources = append(scan.WhyResources, w)
				}
			}
		}

		manifestDir := filepath.Join(compBuildPath, string(layout.ManifestsComponentDir))
		if len(component.Manifests) > 0 {
			err := os.MkdirAll(manifestDir, 0o700)
			if err != nil {
				return FindImagesResult{}, err
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
					return FindImagesResult{}, fmt.Errorf("unable to build the kustomization for %s: %w", path, err)
				}
				manifestPaths = append(manifestPaths, dst)
			}
			// Get all manifest files
			for idx, f := range manifest.Files {
				rel := filepath.Join(string(layout.ManifestsComponentDir), fmt.Sprintf("%s-%d.yaml", manifest.Name, idx))
				dst := filepath.Join(compBuildPath, rel)
				if helpers.IsURL(f) {
					if err := utils.DownloadToFile(ctx, f, dst, component.DeprecatedCosignKeyPath); err != nil {
						return FindImagesResult{}, fmt.Errorf(lang.ErrDownloading, f, err.Error())
					}
				} else {
					if err := helpers.CreatePathAndCopy(filepath.Join(packagePath, f), dst); err != nil {
						return FindImagesResult{}, fmt.Errorf("unable to copy manifest %s: %w", f, err)
					}
				}
				manifestPaths = append(manifestPaths, dst)
			}

			for _, f := range manifestPaths {
				if err := variableConfig.ReplaceTextTemplate(f); err != nil {
					return FindImagesResult{}, err
				}
				// Read the contents of each file
				contents, err := os.ReadFile(f)
				if err != nil {
					return FindImagesResult{}, fmt.Errorf("could not read the file %s: %w", f, err)
				}

				// Break the manifest into separate resources
				yamls, err := utils.SplitYAML(contents)
				if err != nil {
					return FindImagesResult{}, err
				}
				resources = append(resources, yamls...)

				// Check if the --why flag is set and if it is process the manifests
				if opts.Why != "" {
					whyResources, err := findWhyResources(yamls, opts.Why, manifest.Name)
					if err != nil {
						return FindImagesResult{}, fmt.Errorf("could not find why resources for manifest %s: %w", manifest.Name, err)
					}
					for _, w := range whyResources {
						w.ResourceType = ManifestResource
						scan.WhyResources = append(scan.WhyResources, w)
					}
				}
			}
		}

		imgCompStart := time.Now()
		l.Info("looking for images in component", "name", component.Name, "resourcesCount", len(resources))

		for _, resource := range resources {
			if matchedImages, maybeImages, err = processUnstructuredImages(ctx, resource, matchedImages, maybeImages); err != nil {
				return FindImagesResult{}, fmt.Errorf("could not process the Kubernetes resource %s: %w", resource.GetName(), err)
			}
		}

		sortedMatchedImages, sortedExpectedImages := getSortedImages(matchedImages, maybeImages)
		scan.Matches = sortedMatchedImages

		// Handle the "maybes"
		var validMaybeImages []string
		if len(sortedExpectedImages) > 0 {
			for _, image := range sortedExpectedImages {
				if descriptor, err := crane.Head(image, images.WithGlobalInsecureFlag()...); err != nil {
					// Test if this is a real image, if not just quiet log to debug, this is normal
					l.Debug("suspected image does not appear to be valid", "error", err)
				} else {
					// Otherwise, add to the list of images
					l.Debug("imaged digest found", "digest", descriptor.Digest)
					validMaybeImages = append(validMaybeImages, image)
				}
			}
		}
		scan.PotentialMatches = validMaybeImages

		l.Debug("done looking for images in component",
			"name", component.Name,
			"resourcesCount", len(resources),
			"duration", time.Since(imgCompStart))

		if !opts.SkipCosign {
			// Handle cosign artifact lookups
			if len(scan.Matches) > 0 || len(scan.PotentialMatches) > 0 {
				imgStart := time.Now()
				l.Info("looking up cosign artifacts for discovered images", "count", len(scan.Matches)+len(scan.PotentialMatches))

				for _, image := range scan.Matches {
					l.Debug("looking up cosign artifacts for image", "name", image)
					cosignArtifacts, err := utils.GetCosignArtifacts(image)
					if err != nil {
						return FindImagesResult{}, fmt.Errorf("could not lookup the cosign artifacts for image %s: %w", image, err)
					}
					scan.CosignArtifacts = append(scan.CosignArtifacts, cosignArtifacts...)
				}

				for _, image := range scan.PotentialMatches {
					l.Debug("looking up cosign artifacts for image", "name", image)
					cosignArtifacts, err := utils.GetCosignArtifacts(image)
					if err != nil {
						return FindImagesResult{}, fmt.Errorf("could not lookup the cosign artifacts for image %s: %w", image, err)
					}
					scan.CosignArtifacts = append(scan.CosignArtifacts, cosignArtifacts...)
				}
				l.Debug("done looking up cosign artifacts for discovered images", "duration", time.Since(imgStart))
			}
		}

		componentImageScans = append(componentImageScans, scan)
	}

	if opts.Why != "" {
		var foundWhyResource bool
		for _, componentImageScan := range componentImageScans {
			if len(componentImageScan.WhyResources) > 0 {
				foundWhyResource = true
			}
		}
		if !foundWhyResource {
			return FindImagesResult{}, fmt.Errorf("image %s not found in any charts or manifests", opts.Why)
		}
	}

	return FindImagesResult{ComponentImageScans: componentImageScans}, nil
}

// processUnstructuredImages processes a Kubernetes resource and extracts container images
func processUnstructuredImages(ctx context.Context, resource *unstructured.Unstructured, matchedImages, maybeImages map[string]bool) (map[string]bool, map[string]bool, error) {
	l := logger.From(ctx)
	contents := resource.UnstructuredContent()
	b, err := resource.MarshalJSON()
	if err != nil {
		return nil, nil, err
	}

	switch resource.GetKind() {
	case "Pod":
		var pod corev1.Pod
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &pod); err != nil {
			return nil, nil, fmt.Errorf("could not parse pod: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, pod.Spec)

	case "CronJob":
		var cronJob batchv1.CronJob
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &cronJob); err != nil {
			return nil, nil, fmt.Errorf("could not parse cronjob: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, cronJob.Spec.JobTemplate.Spec.Template.Spec)

	case "ReplicationController":
		var rc corev1.ReplicationController
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &rc); err != nil {
			return nil, nil, fmt.Errorf("could not parse replicationcontroller: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, rc.Spec.Template.Spec)
	case "Deployment":
		var deployment v1.Deployment
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &deployment); err != nil {
			return nil, nil, fmt.Errorf("could not parse deployment: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, deployment.Spec.Template.Spec)

	case "DaemonSet":
		var daemonSet v1.DaemonSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &daemonSet); err != nil {
			return nil, nil, fmt.Errorf("could not parse daemonset: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, daemonSet.Spec.Template.Spec)

	case "StatefulSet":
		var statefulSet v1.StatefulSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &statefulSet); err != nil {
			return nil, nil, fmt.Errorf("could not parse statefulset: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, statefulSet.Spec.Template.Spec)

	case "ReplicaSet":
		var replicaSet v1.ReplicaSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &replicaSet); err != nil {
			return nil, nil, fmt.Errorf("could not parse replicaset: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, replicaSet.Spec.Template.Spec)

	case "Job":
		var job batchv1.Job
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &job); err != nil {
			return nil, nil, fmt.Errorf("could not parse job: %w", err)
		}
		matchedImages = appendToImageMap(matchedImages, job.Spec.Template.Spec)

	default:
		// Capture any custom images
		matches := imageCheck.FindAllStringSubmatch(string(b), -1)
		for _, group := range matches {
			l.Debug("found unknown match", "kind", resource.GetKind(), "value", group[1])
			matchedImages[group[1]] = true
		}
	}

	// Capture "maybe images" for all kinds
	matches := imageFuzzyCheck.FindAllStringSubmatch(string(b), -1)
	for _, group := range matches {
		l.Debug("found possible fuzzy match", "kind", resource.GetKind(), "value", group[1])
		maybeImages[group[1]] = true
	}

	return matchedImages, maybeImages, nil
}

// appendToImageMap adds container images to the image map
func appendToImageMap(imgMap map[string]bool, pod corev1.PodSpec) map[string]bool {
	for _, container := range pod.InitContainers {
		if reference.ReferenceRegexp.MatchString(container.Image) {
			imgMap[container.Image] = true
		}
	}
	for _, container := range pod.Containers {
		if reference.ReferenceRegexp.MatchString(container.Image) {
			imgMap[container.Image] = true
		}
	}
	for _, container := range pod.EphemeralContainers {
		if reference.ReferenceRegexp.MatchString(container.Image) {
			imgMap[container.Image] = true
		}
	}
	return imgMap
}

// getSortedImages returns sorted slices of matched and maybe images
func getSortedImages(matchedImages map[string]bool, maybeImages map[string]bool) ([]string, []string) {
	sortedMatchedImages := sort.StringSlice{}
	for image := range matchedImages {
		sortedMatchedImages = append(sortedMatchedImages, image)
	}
	sort.Sort(sortedMatchedImages)

	sortedMaybeImages := sort.StringSlice{}
	for image := range maybeImages {
		if matchedImages[image] {
			continue
		}
		sortedMaybeImages = append(sortedMaybeImages, image)
	}
	sort.Sort(sortedMaybeImages)

	return sortedMatchedImages, sortedMaybeImages
}

func findWhyResources(resources []*unstructured.Unstructured, whyImage, resourceName string) ([]Resource, error) {
	var whyResources []Resource
	for _, resource := range resources {
		b, err := yaml.Marshal(resource.Object)
		if err != nil {
			return nil, err
		}
		yaml := string(b)
		if strings.Contains(yaml, whyImage) {
			why := Resource{
				Content: yaml,
				Name:    resourceName,
			}
			whyResources = append(whyResources, why)
		}
	}
	return whyResources, nil
}
