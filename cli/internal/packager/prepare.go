package packager

import (
	"fmt"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/types"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"regexp"
	"sort"
)

var matchedImages []string

func FindImages() {

	// Load the given zarf package
	if err := config.LoadConfig("zarf.yaml"); err != nil {
		message.Fatal(err, "Unable to read the zarf.yaml file")
	}

	tempPath := createPaths()
	components := config.GetComponents()

	for _, component := range components {

		matchedImages = []string{}

		if len(component.Charts)+len(component.Manifests) < 1 {
			// Skip if it doesn't have what we need
			continue
		}

		// Only process helm charts and raw manifests
		strippedComponent := types.ZarfComponent{
			Charts:    component.Charts,
			Manifests: component.Manifests,
		}

		// keep things DRY by using the package creator
		addComponent(tempPath, strippedComponent)

		var resources []*unstructured.Unstructured

		for _, chart := range component.Charts {
			// Generate helm templates to pass to gitops engine
			template, err := helm.TemplateChart(helm.ChartOptions{
				BasePath: tempPath.components,
				Chart:    chart,
			})

			if err != nil {
				message.Errorf(err, "Problem rendering the helm template for %s", chart.Url)
				continue
			}

			// Break the template into separate resources
			yamls, _ := k8s.SplitYAML([]byte(template))
			for _, yaml := range yamls {
				resources = append(resources, yaml)
			}

		}

		for _, manifest := range component.Manifests {
			for _, file := range manifest.Files {
				// Read the contents of each file
				contents, err := os.ReadFile(file)
				if err != nil {
					message.Errorf(err, "Unable to read the file %s", file)
					continue
				}

				// Break the manifest into separate resources
				yamls, _ := k8s.SplitYAML(contents)
				for _, yaml := range yamls {
					resources = append(resources, yaml)
				}
			}
		}

		var imageSanityCheck = regexp.MustCompile(`(?mi)"image":"([^"]+)"`)

		for _, resource := range resources {
			contents := resource.UnstructuredContent()
			json, _ := resource.MarshalJSON()

			switch resource.GetKind() {
			case "Deployment":
				var deployment v1.Deployment
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &deployment); err != nil {
					message.Errorf(err, "Unable to parse deployment")
					continue
				}
				processPod(deployment.Spec.Template.Spec)

			case "DaemonSet":
				var daemonSet v1.DaemonSet
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &daemonSet); err != nil {
					message.Errorf(err, "Unable to parse daemonset")
					continue
				}
				processPod(daemonSet.Spec.Template.Spec)

			case "StatefulSet":
				var statefulSet v1.StatefulSet
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &statefulSet); err != nil {
					message.Errorf(err, "Unable to parse statefulset")
					continue
				}
				processPod(statefulSet.Spec.Template.Spec)

			case "ReplicaSet":
				var replicaSet v1.ReplicaSet
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(contents, &replicaSet); err != nil {
					message.Errorf(err, "Unable to parse replicaset")
					continue
				}
				processPod(replicaSet.Spec.Template.Spec)

			default:
				// Capture any custom images
				matches := imageSanityCheck.FindAllStringSubmatch(string(json), -1)
				for i := range matches {
					message.Info(matches[i][1])
					matchedImages = append(matchedImages, matches[i][1])
				}
			}

		}

		fmt.Println(fmt.Sprintf("      # %s - %s", config.GetMetaData().Name, component.Name))
		uniqueImages := sort.StringSlice(removeDuplicates(matchedImages))
		sort.Sort(uniqueImages)
		for _, image := range uniqueImages {
			fmt.Println("      - " + image)
		}
		fmt.Println()
	}

}

func processPod(pod corev1.PodSpec) {
	for _, container := range pod.InitContainers {
		// Add image for each init container
		matchedImages = append(matchedImages, container.Image)
	}
	for _, container := range pod.Containers {
		// Add image for each regular container
		matchedImages = append(matchedImages, container.Image)
	}
}
