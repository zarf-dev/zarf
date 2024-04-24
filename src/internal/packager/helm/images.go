// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/goccy/go-yaml"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// ChartImages captures the structure of the helm.sh/images annotation within the Helm chart.
type ChartImages []struct {
	// Name of the image.
	Name string `yaml:"name"`
	// Image with tag.
	Image string `yaml:"image"`
	// Condition specifies the values to determine if the image is included or not.
	Condition string `yaml:"condition"`
	// Dependency is the subchart that contains the image, if empty its the parent chart.
	Dependency string `yaml:"dependency"`
}

// FindAnnotatedImagesForChart attempts to parse any image annotations found in a chart archive or directory.
func FindAnnotatedImagesForChart(chartPath string, values chartutil.Values) (images []string, err error) {
	// Load a new chart.
	chart, err := loader.Load(chartPath)
	if err != nil {
		return images, err
	}
	values = helpers.MergeMapRecursive(chart.Values, values)

	imageAnnotation := chart.Metadata.Annotations["helm.sh/images"]

	var chartImages ChartImages

	err = yaml.Unmarshal([]byte(imageAnnotation), &chartImages)
	if err != nil {
		return images, err
	}

	for _, i := range chartImages {
		// Only include the image if the current values/condition specify it should be included
		if i.Condition != "" {
			value, err := values.PathValue(i.Condition)
			message.Debugf("%#v - %#v - %#v\n", value, i.Condition, err)
			// We intentionally ignore the error here because the key could be missing from the values.yaml
			if err == nil && value == true {
				images = append(images, i.Image)
			}
		} else {
			images = append(images, i.Image)
		}
	}

	return images, nil
}
