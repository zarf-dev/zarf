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
	chart      types.ZarfChart
	chartPath  string
	valuesPath string

	cfg       *types.PackagerConfig
	component types.ZarfComponent
	cluster   *cluster.Cluster

	kubeVersion string

	chartOverride *chart.Chart
	valueOverride map[string]any

	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

// New returns a new Helm config struct.
func New(chart types.ZarfChart, chartPath string, valuesPath string) *Helm {
	return &Helm{
		chart:      chart,
		chartPath:  chartPath,
		valuesPath: valuesPath,
	}
}

// NewClusterOnly returns a new Helm config struct geared toward interacting with the cluster (not packages)
func NewClusterOnly(cfg *types.PackagerConfig, cluster *cluster.Cluster) *Helm {
	return &Helm{
		cfg:     cfg,
		cluster: cluster,
	}
}

// TODO: (@WSTARR) - How to handle?
// NewFromManifest returns a new Helm config struct geared toward interacting with Zarf Manifests instead of Charts
// func NewFromManifest(manifest types.ZarfManifest) *Helm {
// 	return &Helm{
// 		chartPath: chartPath,
// 	}
// }

// WithDeployInfo adds the necessary information to deploy a given chart
func (h *Helm) WithDeployInfo(component types.ZarfComponent, cfg *types.PackagerConfig, cluster *cluster.Cluster) {
	h.component = component
	h.cfg = cfg
	h.cluster = cluster
}

// WithKubeVersion sets the Kube version for templating the chart
func (h *Helm) WithKubeVersion(kubeVersion string) {
	h.kubeVersion = kubeVersion
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart types.ZarfChart) string {
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}
