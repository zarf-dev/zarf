package k8s

import (
	"fmt"
	"sort"
	"time"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	corev1 "k8s.io/api/core/v1"
)

type ImageMap map[string]bool

func GetAllImages() ([]string, error) {
	var images []string
	var err error
	timeout := time.After(5 * time.Minute)

	for {
		// delay check 3 seconds
		time.Sleep(2 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			message.Debug("get image list timed-out")
			return images, nil

		// after delay, try running
		default:
			// If no images or an error, log and loop
			if images, err = GetImages(corev1.NamespaceAll); len(images) < 1 || err != nil {
				message.Debug(err)
			} else {
				// Otherwise, return the image list
				return images, nil
			}
		}
	}
}

func GetImages(namespace string) ([]string, error) {
	images := make(ImageMap)

	pods, err := GetPods(namespace)
	if err != nil {
		return []string{}, fmt.Errorf("unable to get the list of pods in the cluster")
	}

	for _, pod := range pods.Items {
		images = BuildImageMap(images, pod.Spec)
	}

	return SortImages(images, nil), nil
}

// BuildImageMap looks for init container, ephemeral and regular container images
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

func SortImages(images ImageMap, compareWith ImageMap) []string {
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
