package k8s

import (
	"fmt"
	"sort"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/message"
	corev1 "k8s.io/api/core/v1"
)

type ImageMap map[string]bool
type ImageNodeMap map[string][]string

// GetAllImages returns a list of images and their nodes found in pods in the cluster.
func GetAllImages() (ImageNodeMap, error) {
	timeout := time.After(5 * time.Minute)

	for {
		// delay check 2 seconds
		time.Sleep(2 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			return nil, fmt.Errorf("get image list timed-out")

		// after delay, try running
		default:
			// If no images or an error, log and loop
			if images, err := GetImagesWithNodes(corev1.NamespaceAll); len(images) < 1 || err != nil {
				message.Debug(err)
			} else {
				// Otherwise, return the image list
				return images, nil
			}
		}
	}
}

// GetImagesWithNodes returns all images and their nodes in a given namespace.
func GetImagesWithNodes(namespace string) (ImageNodeMap, error) {
	result := make(ImageNodeMap)

	pods, err := GetPods(namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get the list of pods in the cluster")
	}

	for _, pod := range pods.Items {
		node := pod.Spec.NodeName
		for _, container := range pod.Spec.InitContainers {
			result[container.Image] = append(result[container.Image], node)
		}
		for _, container := range pod.Spec.Containers {
			result[container.Image] = append(result[container.Image], node)
		}
		for _, container := range pod.Spec.EphemeralContainers {
			result[container.Image] = append(result[container.Image], node)
		}
	}

	return result, nil
}

// BuildImageMap looks for init container, ephemeral and regular container images.
func BuildImageMap(images ImageMap, pod corev1.PodSpec) ImageMap {
	for _, container := range pod.InitContainers {
		images[container.Image] = true
	}
	for _, container := range pod.Containers {
		images[container.Image] = true
	}
	for _, container := range pod.EphemeralContainers {
		images[container.Image] = true
	}
	return images
}

// SortImages returns a sorted list of images.
func SortImages(images, compareWith ImageMap) []string {
	sortedImages := sort.StringSlice{}
	for image := range images {
		if !compareWith[image] || compareWith == nil {
			// Check compareWith, if it exists only add if not in that list
			sortedImages = append(sortedImages, image)
		}
	}
	sort.Sort(sortedImages)
	return sortedImages
}
