// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package retrieve_images contains the logic for automatically populating the images field
package retrieve_images

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/defenseunicorns/zarf/src/types/extensions"
)

func Run(tmpPaths types.ComponentPaths, component types.ZarfComponent, packageConfig *types.PackagerConfig) (types.ZarfComponent, error) {
	var err error

	if len(component.Extensions.RetrieveImages.FromGitChart) > 0 {
		gitChartImages, _ := retrieveImageFromGitCharts(tmpPaths, component.Extensions.RetrieveImages.FromGitChart, packageConfig)
		component.Images = mergeImages(component.Images, gitChartImages)
	}

	return component, err
}

func mergeImages(currentImages []string, newImages []string) []string {
	imageMap := make(map[string]bool)

	for _, image := range currentImages {
		imageMap[image] = true
	}

	for _, image := range newImages {
		imageMap[image] = true
	}

	mergedImages := make([]string, 0, len(imageMap))
	for image := range imageMap {
		mergedImages = append(mergedImages, image)
	}

	return mergedImages
}

func retrieveImageFromGitCharts(tmpPaths types.ComponentPaths, gitChartDefs []extensions.FromGitChart, packageConfig *types.PackagerConfig) ([]string, error) {
	charts := make([]types.ZarfChart, 0, len(gitChartDefs))
	var retrievedImages []string

	for index, gitChartDef := range gitChartDefs {
		charts = append(charts, types.ZarfChart{
			Name:    fmt.Sprint(index),
			URL:     gitChartDef.Url,
			Version: gitChartDef.Tag, // TODO - handle branch
			GitPath: strings.TrimPrefix(gitChartDef.Path, "/"),
		})
	}

	chartOverrides := make(map[string]string)

	_ = utils.CreateDirectory(tmpPaths.Charts, 0700)
	_ = utils.CreateDirectory(tmpPaths.Values, 0700)

	for _, chart := range charts {
		helmCfg := helm.Helm{
			Chart: chart,
			Cfg:   packageConfig,
		}

		helmCfg.Cfg.State = types.ZarfState{}

		path, err := helmCfg.PackageChartFromGit(tmpPaths.Charts)
		if err != nil {
			return []string{}, fmt.Errorf("unable to download chart from git repo (%s): %w", chart.URL, err)
		}
		// track the actual chart path
		chartOverrides[chart.Name] = path

		// Generate helm templates to pass to gitops engine
		helmCfg = helm.Helm{
			BasePath:          tmpPaths.Base,
			Chart:             chart,
			ChartLoadOverride: chartOverrides[chart.Name],
		}
		_, values, err := helmCfg.TemplateChart()

		if err != nil {
			return []string{}, fmt.Errorf("problem rendering the helm template for %s: %s", chart.URL, err.Error())
		}

		var chartTarball string
		if overridePath, ok := chartOverrides[chart.Name]; ok {
			chartTarball = overridePath
		} else {
			chartTarball = helm.StandardName(tmpPaths.Charts, helmCfg.Chart) + ".tgz"
		}

		annotatedImages, err := helm.FindAnnotatedImagesForChart(chartTarball, values)
		if err != nil {
			return []string{}, fmt.Errorf("problem looking for image annotations for %s: %s", chart.URL, err.Error())
		}
		retrievedImages = append(retrievedImages, annotatedImages...)
	}

	return retrievedImages, nil
}
