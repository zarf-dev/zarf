// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml"
	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
)

// ChartImage represents a single image entry in the helm.sh/images annotation.
type ChartImage struct {
	// Name of the image.
	Name string `yaml:"name"`
	// Image with tag.
	Image string `yaml:"image"`
	// Condition specifies the values to determine if the image is included or not.
	Condition string `yaml:"condition"`
	// Dependency is the subchart that contains the image, if empty its the parent chart.
	Dependency string `yaml:"dependency"`
}

// ChartImages captures the structure of the helm.sh/images annotation within the Helm chart.
type ChartImages []ChartImage

// FindAnnotatedImagesForChart attempts to parse any image annotations found in a chart archive or directory.
func FindAnnotatedImagesForChart(chartPath string, values common.Values) (images []string, err error) {
	// Load a new chart.
	chart, err := loader.Load(chartPath)
	if err != nil {
		return images, err
	}
	values = helpers.MergeMapRecursive(chart.Values, values)

	// Use a map to deduplicate images across parent chart and all dependencies
	imageSet := make(map[string]bool)

	// Recursively find all images in the chart and its dependencies
	findImagesRecursive(chart, values, imageSet)

	// Convert set to slice
	for image := range imageSet {
		images = append(images, image)
	}

	return images, nil
}

// findImagesRecursive recursively finds images in a chart and its dependencies.
func findImagesRecursive(c *chartv2.Chart, values common.Values, imageSet map[string]bool) {
	// Process current chart's annotations
	if imageAnnotation, ok := c.Metadata.Annotations["helm.sh/images"]; ok && imageAnnotation != "" {
		var chartImages ChartImages
		if err := yaml.Unmarshal([]byte(imageAnnotation), &chartImages); err == nil {
			for _, i := range chartImages {
				if shouldIncludeImage(i, values) {
					imageSet[i.Image] = true
				}
			}
		}
	}

	// Process dependencies recursively
	for _, depChart := range c.Dependencies() {
		var subchartValues common.Values
		if depChart.Name() != "" {
			// Try to access subchart values using the dependency name as the key
			if depValues, ok := values[depChart.Name()].(map[string]interface{}); ok {
				subchartValues = common.Values(depValues)
			} else {
				// If no specific values for this subchart, use empty values
				subchartValues = common.Values{}
			}
		} else {
			subchartValues = common.Values{}
		}

		findImagesRecursive(depChart, subchartValues, imageSet)
	}
}

// shouldIncludeImage determines if an image should be included based on its condition.
func shouldIncludeImage(img ChartImage, values common.Values) bool {
	if img.Condition == "" {
		return true
	}

	value, err := values.PathValue(img.Condition)
	return err == nil && value == true
}
