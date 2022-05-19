package packager

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var matchedImages k8s.ImageMap
var maybeImages k8s.ImageMap

// FindImages iterates over a zarf.yaml and attempts to parse any images
func FindImages(repoHelmChartPath string) {

	// Load the given zarf package
	if err := config.LoadConfig("zarf.yaml"); err != nil {
		message.Fatal(err, "Unable to read the zarf.yaml file")
	}

	components := config.GetComponents()
	tempPath := createPaths()
	defer tempPath.clean()

	for _, component := range components {

		// matchedImages holds the collection of images, reset per-component
		matchedImages = make(k8s.ImageMap)
		maybeImages = make(k8s.ImageMap)

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

				// If a repo helmchartpath is specified,
				component.Charts = append(component.Charts, types.ZarfChart{
					Name:    repo,
					Url:     matches[0],
					Version: matches[1],
					GitPath: repoHelmChartPath,
				})
			}
		}

		// resources are a slice of generic structs that represent parsed K8s resources
		var resources []*unstructured.Unstructured

		componentPath := createComponentPaths(tempPath.components, component)
		chartNames := make(map[string]string)

		if len(component.Charts) > 0 {
			_ = utils.CreateDirectory(componentPath.charts, 0700)
			_ = utils.CreateDirectory(componentPath.values, 0700)
			gitUrlRegex := regexp.MustCompile(`\.git$`)

			for _, chart := range component.Charts {
				isGitURL := gitUrlRegex.MatchString(chart.Url)
				if isGitURL {
					path := helm.DownloadChartFromGit(chart, componentPath.charts)
					// track the actual chart path
					chartNames[chart.Name] = path
				} else {
					helm.DownloadPublishedChart(chart, componentPath.charts)
				}

				for idx, path := range chart.ValuesFiles {
					chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
					utils.CreatePathAndCopy(path, chartValueName)
				}

				var override string
				var ok bool

				if override, ok = chartNames[chart.Name]; ok {
					chart.Name = "dummy"
				}

				// Generate helm templates to pass to gitops engine
				template, err := helm.TemplateChart(helm.ChartOptions{
					BasePath:          componentPath.base,
					Chart:             chart,
					ChartLoadOverride: override,
				})

				if err != nil {
					message.Errorf(err, "Problem rendering the helm template for %s", chart.Url)
					continue
				}

				// Break the template into separate resources
				yamls, _ := k8s.SplitYAML([]byte(template))
				resources = append(resources, yamls...)
			}
		}

		if len(component.Manifests) > 0 {
			if err := utils.CreateDirectory(componentPath.manifests, 0700); err != nil {
				message.Errorf(err, "Unable to create the manifest path %s", componentPath.manifests)
			}

			for _, manifest := range component.Manifests {
				for idx, kustomization := range manifest.Kustomizations {
					// Generate manifests from kustomizations and place in the package
					destination := fmt.Sprintf("%s/kustomization-%s-%d.yaml", componentPath.manifests, manifest.Name, idx)
					if err := kustomize.BuildKustomization(kustomization, destination, manifest.KustomizeAllowAnyDirectory); err != nil {
						message.Errorf(err, "unable to build the kustomization for %s", kustomization)
					} else {
						manifest.Files = append(manifest.Files, destination)
					}
				}

				// Get all manifest files
				for _, file := range manifest.Files {
					// Read the contents of each file
					contents, err := os.ReadFile(file)
					if err != nil {
						message.Errorf(err, "Unable to read the file %s", file)
						continue
					}

					// Break the manifest into separate resources
					yamls, _ := k8s.SplitYAML(contents)
					resources = append(resources, yamls...)
				}
			}
		}

		for _, resource := range resources {
			if err := processUnstructured(resource); err != nil {
				message.Errorf(err, "Problem processing K8s resource %s", resource.GetName())
			}
		}

		if sortedImages := k8s.SortImages(matchedImages, nil); len(sortedImages) > 0 {
			// Log the header comment
			fmt.Printf("      # %s - %s\n", config.GetMetaData().Name, component.Name)
			for _, image := range sortedImages {
				// Use print because we want this dumped to stdout
				fmt.Println("      - " + image)
			}
		}

		// Handle the "maybes"
		if sortedImages := k8s.SortImages(maybeImages, matchedImages); len(sortedImages) > 0 {
			var realImages []string
			for _, image := range sortedImages {
				if descriptor, err := crane.Head(image, config.GetCraneOptions()); err != nil {
					// Test if this is a real image, if not just quiet log to debug, this is normal
					message.Debugf("Suspected image does not appear to be valid: %w", err)
				} else {
					// Otherwise, add to the list of images
					message.Debugf("Imaged digest found: %s", descriptor.Digest)
					realImages = append(realImages, image)
				}
			}

			if len(realImages) > 0 {
				fmt.Printf("      # Possible images - %s - %s\n", config.GetMetaData().Name, component.Name)
				for _, image := range realImages {
					fmt.Println("      - " + image)
				}
			}
		}
	}
}

func processUnstructured(resource *unstructured.Unstructured) error {
	var imageSanityCheck = regexp.MustCompile(`(?mi)"image":"([^"]+)"`)
	var imageFuzzyCheck = regexp.MustCompile(`(?mi)"([a-z0-9\-./]+:[\w][\w.\-]{0,127})"`)
	var json string

	contents := resource.UnstructuredContent()
	bytes, _ := resource.MarshalJSON()
	json = string(bytes)

	message.Debug()

	switch resource.GetKind() {
	case "Deployment":
		var deployment v1.Deployment
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &deployment); err != nil {
			return fmt.Errorf("could not parse deployment: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, deployment.Spec.Template.Spec)

	case "DaemonSet":
		var daemonSet v1.DaemonSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &daemonSet); err != nil {
			return fmt.Errorf("could not parse daemonset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, daemonSet.Spec.Template.Spec)

	case "StatefulSet":
		var statefulSet v1.StatefulSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &statefulSet); err != nil {
			return fmt.Errorf("could not parse statefulset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, statefulSet.Spec.Template.Spec)

	case "ReplicaSet":
		var replicaSet v1.ReplicaSet
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &replicaSet); err != nil {
			return fmt.Errorf("could not parse replicaset: %w", err)
		}
		matchedImages = k8s.BuildImageMap(matchedImages, replicaSet.Spec.Template.Spec)

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
	return nil
}
