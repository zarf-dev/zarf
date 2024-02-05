// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// FindImages iterates over a Zarf.yaml and attempts to parse any images.
func (p *Packager) FindImages() (imgMap map[string][]string, err error) {
	repoHelmChartPath := p.cfg.FindImagesOpts.RepoHelmChartPath
	kubeVersionOverride := p.cfg.FindImagesOpts.KubeVersionOverride

	imagesMap := make(map[string][]string)
	erroredCharts := []string{}
	erroredCosignLookups := []string{}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if err := os.Chdir(p.cfg.CreateOpts.BaseDir); err != nil {
		return nil, fmt.Errorf("unable to access directory '%s': %w", p.cfg.CreateOpts.BaseDir, err)
	}
	message.Note(fmt.Sprintf("Using build directory %s", p.cfg.CreateOpts.BaseDir))

	if err = p.readZarfYAML(layout.ZarfYAML); err != nil {
		return nil, fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
	}

	if err := p.composeComponents(); err != nil {
		return nil, err
	}

	for _, warning := range p.warnings {
		message.Warn(warning)
	}

	// After components are composed, template the active package
	if err := p.fillActiveTemplate(); err != nil {
		return nil, fmt.Errorf("unable to fill values in template: %s", err.Error())
	}

	for _, component := range p.cfg.Pkg.Components {
		if len(component.Repos) > 0 && repoHelmChartPath == "" {
			message.Note("This Zarf package contains git repositories, " +
				"if any repos contain helm charts you want to template and " +
				"search for images, make sure to specify the helm chart path " +
				"via the --repo-chart-path flag")
		}
	}

	componentDefinition := "\ncomponents:\n"

	if err := p.setVariableMapInConfig(); err != nil {
		return nil, fmt.Errorf(lang.PkgErrsetVariableMap, err)
	}

	for _, component := range p.cfg.Pkg.Components {

		if len(component.Charts)+len(component.Manifests)+len(component.Repos) < 1 {
			// Skip if it doesn't have what we need
			continue
		}

		if repoHelmChartPath != "" {
			// Also process git repos that have helm charts
			for _, repo := range component.Repos {
				matches := strings.Split(repo, "@")
				if len(matches) < 2 {
					message.Warnf("Cannot convert git repo %s to helm chart without a version tag", repo)
					continue
				}

				// Trim the first char to match how the packager expects it, this is messy,need to clean up better
				repoHelmChartPath = strings.TrimPrefix(repoHelmChartPath, "/")

				// If a repo helm chart path is specified,
				component.Charts = append(component.Charts, types.ZarfChart{
					Name:    repo,
					URL:     matches[0],
					Version: matches[1],
					GitPath: repoHelmChartPath,
				})
			}
		}

		// matchedImages holds the collection of images, reset per-component
		matchedImages := make(k8s.ImageMap)
		maybeImages := make(k8s.ImageMap)

		// resources are a slice of generic structs that represent parsed K8s resources
		var resources []*unstructured.Unstructured

		componentPaths, err := p.layout.Components.Create(component)
		if err != nil {
			return nil, err
		}
		values, err := template.Generate(p.cfg)
		if err != nil {
			return nil, fmt.Errorf("unable to generate template values")
		}
		values.SetState(&types.ZarfState{})
		values.SetRegistry(p.cfg.FindImagesOpts.RegistryOverride)
		for _, chart := range component.Charts {

			helmCfg := helm.New(
				chart,
				componentPaths.Charts,
				componentPaths.Values,
				helm.WithKubeVersion(kubeVersionOverride),
				helm.WithPackageConfig(p.cfg),
			)

			err = helmCfg.PackageChart(component.DeprecatedCosignKeyPath)
			if err != nil {
				return nil, fmt.Errorf("unable to package the chart %s: %s", chart.Name, err.Error())
			}

			manifests, _ := utils.RecursiveFileList(componentPaths.Values, nil, false)
			for _, manifest := range manifests {
				if err := values.Apply(component, manifest, false); err != nil {
					return nil, err
				}
			}

			// Generate helm templates for this chart
			chartTemplate, chartValues, err := helmCfg.TemplateChart()

			if err != nil {
				erroredCharts = append(erroredCharts, chart.Name)
				continue
			}

			// Break the template into separate resources
			yamls, _ := utils.SplitYAML([]byte(chartTemplate))
			resources = append(resources, yamls...)

			chartTarball := helm.StandardName(componentPaths.Charts, chart) + ".tgz"

			annotatedImages, err := helm.FindAnnotatedImagesForChart(chartTarball, chartValues)
			if err != nil {
				message.WarnErrf(err, "Problem looking for image annotations for %s: %s", chart.URL, err.Error())
				erroredCharts = append(erroredCharts, chart.URL)
				continue
			}
			for _, image := range annotatedImages {
				matchedImages[image] = true
			}
		}

		for _, manifest := range component.Manifests {
			for idx, k := range manifest.Kustomizations {
				// Generate manifests from kustomizations and place in the package
				kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
				destination := filepath.Join(componentPaths.Manifests, kname)
				if err := kustomize.Build(k, destination, manifest.KustomizeAllowAnyDirectory); err != nil {
					return nil, fmt.Errorf("unable to build the kustomization for %s: %s", k, err.Error())
				}
				manifest.Files = append(manifest.Files, destination)
			}
			// Get all manifest files
			for idx, f := range manifest.Files {
				if helpers.IsURL(f) {
					mname := fmt.Sprintf("manifest-%s-%d.yaml", manifest.Name, idx)
					destination := filepath.Join(componentPaths.Manifests, mname)
					if err := utils.DownloadToFile(f, destination, component.DeprecatedCosignKeyPath); err != nil {
						return nil, fmt.Errorf(lang.ErrDownloading, f, err.Error())
					}
					f = destination
				} else {
					newDestination := filepath.Join(componentPaths.Manifests, f)
					utils.CreatePathAndCopy(f, newDestination)
					f = newDestination
				}

				if err := values.Apply(component, f, true); err != nil {
					return nil, err
				}
				// Read the contents of each file
				contents, err := os.ReadFile(f)
				if err != nil {
					message.WarnErrf(err, "Unable to read the file %s", f)
					continue
				}

				// Break the manifest into separate resources
				contentString := string(contents)
				message.Debugf("%s", contentString)
				yamls, _ := utils.SplitYAML(contents)
				resources = append(resources, yamls...)
			}
		}

		spinner := message.NewProgressSpinner("Looking for images in component %q across %d resources", component.Name, len(resources))
		defer spinner.Stop()

		for _, resource := range resources {
			if matchedImages, maybeImages, err = p.processUnstructuredImages(resource, matchedImages, maybeImages); err != nil {
				message.WarnErrf(err, "Problem processing K8s resource %s", resource.GetName())
			}
		}

		if sortedImages := k8s.SortImages(matchedImages, nil); len(sortedImages) > 0 {
			// Log the header comment
			componentDefinition += fmt.Sprintf("\n  - name: %s\n    images:\n", component.Name)
			for _, image := range sortedImages {
				// Use print because we want this dumped to stdout
				imagesMap[component.Name] = append(imagesMap[component.Name], image)
				componentDefinition += fmt.Sprintf("      - %s\n", image)
			}
		}

		// Handle the "maybes"
		if sortedImages := k8s.SortImages(maybeImages, matchedImages); len(sortedImages) > 0 {
			var validImages []string
			for _, image := range sortedImages {
				if descriptor, err := crane.Head(image, config.GetCraneOptions(config.CommonOptions.Insecure)...); err != nil {
					// Test if this is a real image, if not just quiet log to debug, this is normal
					message.Debugf("Suspected image does not appear to be valid: %#v", err)
				} else {
					// Otherwise, add to the list of images
					message.Debugf("Imaged digest found: %s", descriptor.Digest)
					validImages = append(validImages, image)
				}
			}

			if len(validImages) > 0 {
				componentDefinition += fmt.Sprintf("      # Possible images - %s - %s\n", p.cfg.Pkg.Metadata.Name, component.Name)
				for _, image := range validImages {
					imagesMap[component.Name] = append(imagesMap[component.Name], image)
					componentDefinition += fmt.Sprintf("      - %s\n", image)
				}
			}
		}

		spinner.Success()

		// Handle cosign artifact lookups
		if len(imagesMap[component.Name]) > 0 {
			var cosignArtifactList []string
			spinner := message.NewProgressSpinner("Looking up cosign artifacts for discovered images (0/%d)", len(imagesMap[component.Name]))
			defer spinner.Stop()

			for idx, image := range imagesMap[component.Name] {
				spinner.Updatef("Looking up cosign artifacts for discovered images (%d/%d)", idx+1, len(imagesMap[component.Name]))
				cosignArtifacts, err := utils.GetCosignArtifacts(image)
				if err != nil {
					message.WarnErrf(err, "Problem looking up cosign artifacts for %s: %s", image, err.Error())
					erroredCosignLookups = append(erroredCosignLookups, image)
				}
				cosignArtifactList = append(cosignArtifactList, cosignArtifacts...)
			}

			spinner.Success()

			if len(cosignArtifactList) > 0 {
				imagesMap[component.Name] = append(imagesMap[component.Name], cosignArtifactList...)
				componentDefinition += fmt.Sprintf("      # Cosign artifacts for images - %s - %s\n", p.cfg.Pkg.Metadata.Name, component.Name)
				for _, cosignArtifact := range cosignArtifactList {
					componentDefinition += fmt.Sprintf("      - %s\n", cosignArtifact)
				}
			}
		}
	}

	fmt.Println(componentDefinition)

	// Return to the original working directory
	if err := os.Chdir(cwd); err != nil {
		return nil, err
	}

	if len(erroredCharts) > 0 || len(erroredCosignLookups) > 0 {
		errMsg := ""
		if len(erroredCharts) > 0 {
			errMsg = fmt.Sprintf("the following charts had errors: %s", erroredCharts)
		}
		if len(erroredCosignLookups) > 0 {
			if errMsg != "" {
				errMsg += "\n"
			}
			errMsg += fmt.Sprintf("the following images errored on cosign lookups: %s", erroredCosignLookups)
		}
		return imagesMap, fmt.Errorf(errMsg)
	}

	return imagesMap, nil
}

func (p *Packager) processUnstructuredImages(resource *unstructured.Unstructured, matchedImages, maybeImages k8s.ImageMap) (k8s.ImageMap, k8s.ImageMap, error) {
	var imageSanityCheck = regexp.MustCompile(`(?mi)"image":"([^"]+)"`)
	var imageFuzzyCheck = regexp.MustCompile(`(?mi)["|=]([a-z0-9\-.\/:]+:[\w.\-]*[a-z\.\-][\w.\-]*)"`)
	var json string

	contents := resource.UnstructuredContent()
	bytes, _ := resource.MarshalJSON()
	json = string(bytes)

	switch resource.GetKind() {
	case "Deployment":
		var deployment v1.Deployment
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &deployment); err != nil {
			return matchedImages, maybeImages, fmt.Errorf("could not parse deployment: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, deployment.Spec.Template.Spec)

	case "DaemonSet":
		var daemonSet v1.DaemonSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &daemonSet); err != nil {
			return matchedImages, maybeImages, fmt.Errorf("could not parse daemonset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, daemonSet.Spec.Template.Spec)

	case "StatefulSet":
		var statefulSet v1.StatefulSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &statefulSet); err != nil {
			return matchedImages, maybeImages, fmt.Errorf("could not parse statefulset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, statefulSet.Spec.Template.Spec)

	case "ReplicaSet":
		var replicaSet v1.ReplicaSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &replicaSet); err != nil {
			return matchedImages, maybeImages, fmt.Errorf("could not parse replicaset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, replicaSet.Spec.Template.Spec)

	case "Job":
		var job batchv1.Job
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &job); err != nil {
			return matchedImages, maybeImages, fmt.Errorf("could not parse job: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, job.Spec.Template.Spec)

	default:
		// Capture any custom images
		matches := imageSanityCheck.FindAllStringSubmatch(json, -1)
		for _, group := range matches {
			message.Debugf("Found unknown match, Kind: %s, Value: %s", resource.GetKind(), group[1])
			matchedImages[group[1]] = true
		}
	}

	// Capture "maybe images" too for all kinds because they might be in unexpected places.... 👀
	matches := imageFuzzyCheck.FindAllStringSubmatch(json, -1)
	for _, group := range matches {
		message.Debugf("Found possible fuzzy match, Kind: %s, Value: %s", resource.GetKind(), group[1])
		maybeImages[group[1]] = true
	}

	return matchedImages, maybeImages, nil
}
