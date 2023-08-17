// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/variables"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
)

// HelmCfg is a config object for working with helm charts.
type HelmCfg struct {
	chart               *types.ZarfChart
	kubeVersionOverride string

	cluster *cluster.Cluster
	state   *types.ZarfState

	pkgMetadata            types.ZarfMetadata
	component              types.ZarfComponent
	componentPaths         types.ComponentPaths
	valueTemplate          *variables.Values
	adoptExistingResources bool

	releaseName   string
	chartOverride *chart.Chart
	valueOverride map[string]any

	settings     *cli.EnvSettings
	actionConfig *action.Configuration
}

// New creates a new Helm config to process a provided chart
func New(chart *types.ZarfChart, kubeVersionOverride string) *HelmCfg {
	return &HelmCfg{
		chart:               chart,
		kubeVersionOverride: kubeVersionOverride,
	}
}

// WithCluster adds cluster and state information to the given helm config
func (h *HelmCfg) WithCluster(cluster *cluster.Cluster, state *types.ZarfState) *HelmCfg {
	h.cluster = cluster
	h.state = state
	return h
}

// WithComponent adds component information to the given helm config
func (h *HelmCfg) WithComponent(pkgMetadata types.ZarfMetadata, component types.ZarfComponent, componentPaths types.ComponentPaths) *HelmCfg {
	h.pkgMetadata = pkgMetadata
	h.component = component
	h.componentPaths = componentPaths
	return h
}

// WithValues adds runtime values to the given helm config
func (h *HelmCfg) WithValues(valueTemplate *variables.Values, adoptExistingResources bool) *HelmCfg {
	h.valueTemplate = valueTemplate
	h.adoptExistingResources = adoptExistingResources
	return h
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart *types.ZarfChart) string {
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}
