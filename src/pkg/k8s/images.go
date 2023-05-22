// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// ImageMap is a map of image/boolean pairs.
type ImageMap map[string]bool

// ImageNodeMap is a map of image/node pairs.
type ImageNodeMap map[string][]string

// GetAllImages returns a list of images and their nodes found in pods in the cluster.
func (k *K8s) GetAllImages(timeoutDuration time.Duration) (ImageNodeMap, error) {
	timeout := time.After(timeoutDuration)

	for {
		// Delay check 2 seconds.
		time.Sleep(2 * time.Second)
		select {

		// On timeout abort.
		case <-timeout:
			return nil, fmt.Errorf("get image list timed-out")

		// After delay, try running.
		default:
			// If no images or an error, log and loop.
			if images, err := k.GetImagesWithNodes(corev1.NamespaceAll); len(images) < 1 || err != nil {
				k.Log("no images found: %w", err)
			} else {
				// Otherwise, return the image list.
				return images, nil
			}
		}
	}
}

// GetImagesWithNodes checks for images on schedulable nodes and returns
// a map of these images and their nodes in a given namespace.
func (k *K8s) GetImagesWithNodes(namespace string) (ImageNodeMap, error) {
	result := make(ImageNodeMap)

	pods, err := k.GetPods(namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get the list of pods in the cluster")
	}

	findImages:
	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		nodeDetails, err := k.GetNode(nodeName)
		if err != nil {
			return nil, fmt.Errorf("unable to get the node %s", pod.Spec.NodeName)
		}

		for _, taint := range nodeDetails.Spec.Taints {
			if (taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute) {
				continue findImages
			}
		}
		for _, container := range pod.Spec.InitContainers {
			result[container.Image] = append(result[container.Image], nodeName)
		}
		for _, container := range pod.Spec.Containers {
			result[container.Image] = append(result[container.Image], nodeName)
		}
		for _, container := range pod.Spec.EphemeralContainers {
			result[container.Image] = append(result[container.Image], nodeName)
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
			// Check compareWith, if it exists only add if not in that list.
			sortedImages = append(sortedImages, image)
		}
	}
	sort.Sort(sortedImages)
	return sortedImages
}
