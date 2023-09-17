// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
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
func (p *Packager) FindImages(baseDir, repoHelmChartPath string, kubeVersionOverride string) (map[string][]string, error) {

	var originalDir string
	imagesMap := make(map[string][]string)

	// Change the working directory if this run has an alternate base dir
	if baseDir != "" {
		originalDir, _ = os.Getwd()
		_ = os.Chdir(baseDir)
		message.Note(fmt.Sprintf("Using base directory %s", baseDir))
	}

	if err := p.readYaml(config.ZarfYAML); err != nil {
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

		componentPath, err := p.createOrGetComponentPaths(component)
		if err != nil {
			return nil, fmt.Errorf("unable to create component paths: %s", err.Error())
		}

		chartOverrides := make(map[string]string)

		if len(component.Charts) > 0 {
			_ = utils.CreateDirectory(componentPath.Charts, 0700)
			_ = utils.CreateDirectory(componentPath.Values, 0700)

			for _, chart := range component.Charts {

				helmCfg := helm.Helm{
					Chart: chart,
					Cfg:   p.cfg,
				}

				helmCfg.Cfg.State = &types.ZarfState{}

				err := helmCfg.PackageChart(componentPath.Charts)
				if err != nil {
					return nil, fmt.Errorf("unable to package the chart %s: %s", chart.URL, err.Error())
				}

				for idx, path := range chart.ValuesFiles {
					dst := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
					if helpers.IsURL(path) {
						if err := utils.DownloadToFile(path, dst, component.DeprecatedCosignKeyPath); err != nil {
							return nil, fmt.Errorf(lang.ErrDownloading, path, err.Error())
						}
					} else {
						if err := utils.CreatePathAndCopy(path, dst); err != nil {
							return nil, fmt.Errorf("unable to copy values file %s: %w", path, err)
						}
					}
				}

				// Generate helm templates to pass to gitops engine
				helmCfg = helm.Helm{
					BasePath:          componentPath.Base,
					Chart:             chart,
					ChartLoadOverride: chartOverrides[chart.Name],
					KubeVersion:       kubeVersionOverride,
				}
				template, values, err := helmCfg.TemplateChart()

				if err != nil {
					message.WarnErrf(err, "Problem rendering the helm template for %s: %s", chart.URL, err.Error())
					continue
				}

				// Break the template into separate resources
				yamls, _ := utils.SplitYAML([]byte(template))
				resources = append(resources, yamls...)

				var chartTarball string
				if overridePath, ok := chartOverrides[chart.Name]; ok {
					chartTarball = overridePath
				} else {
					chartTarball = helm.StandardName(componentPath.Charts, helmCfg.Chart) + ".tgz"
				}

				annotatedImages, err := helm.FindAnnotatedImagesForChart(chartTarball, values)
				if err != nil {
					message.WarnErrf(err, "Problem looking for image annotations for %s: %s", chart.URL, err.Error())
					continue
				}
				for _, image := range annotatedImages {
					matchedImages[image] = true
				}
			}
		}

		if len(component.Manifests) > 0 {
			if err := utils.CreateDirectory(componentPath.Manifests, 0700); err != nil {
				return nil, fmt.Errorf("unable to create the manifest path %s: %s", componentPath.Manifests, err.Error())
			}

			for _, manifest := range component.Manifests {
				for idx, k := range manifest.Kustomizations {
					// Generate manifests from kustomizations and place in the package
					kname := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
					destination := filepath.Join(componentPath.Manifests, kname)
					if err := kustomize.Build(k, destination, manifest.KustomizeAllowAnyDirectory); err != nil {
						return nil, fmt.Errorf("unable to build the kustomization for %s: %s", k, err.Error())
					}
					manifest.Files = append(manifest.Files, destination)
				}
				// Get all manifest files
				for idx, f := range manifest.Files {
					if helpers.IsURL(f) {
						mname := fmt.Sprintf("manifest-%s-%d.yaml", manifest.Name, idx)
						destination := filepath.Join(componentPath.Manifests, mname)
						if err := utils.DownloadToFile(f, destination, component.DeprecatedCosignKeyPath); err != nil {
							return nil, fmt.Errorf(lang.ErrDownloading, f, err.Error())
						}
						f = destination
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
		}

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
					componentDefinition += fmt.Sprintf("      - %s\n", image)
				}
			}
		}
	}

	fmt.Println(componentDefinition)

	// In case the directory was changed, reset to prevent breaking relative target paths
	if originalDir != "" {
		_ = os.Chdir(originalDir)
	}

	return imagesMap, nil
}

func (p *Packager) processUnstructuredImages(resource *unstructured.Unstructured, matchedImages, maybeImages k8s.ImageMap) (k8s.ImageMap, k8s.ImageMap, error) {
	var imageSanityCheck = regexp.MustCompile(`(?mi)"image":"([^"]+)"`)
	var imageFuzzyCheck = regexp.MustCompile(`(?mi)"([a-z0-9\-.\/]+:[\w][\w.\-]{0,127})"`)
	var json string

	contents := resource.UnstructuredContent()
	bytes, _ := resource.MarshalJSON()
	json = string(bytes)

	message.Debug()

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
