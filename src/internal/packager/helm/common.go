// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
)

// Helm is a config object for working with helm charts.
type Helm struct {
	BasePath          string
	Chart             types.ZarfChart
	ReleaseName       string
	ChartLoadOverride string
	ChartOverride     *chart.Chart
	ValueOverride     map[string]any
	Component         types.ZarfComponent
	Cluster           *cluster.Cluster
	Cfg               *types.PackagerConfig
	KubeVersion       string
	Settings          *cli.EnvSettings

	actionConfig *action.Configuration
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart types.ZarfChart) string {
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}
