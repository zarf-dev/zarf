// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
)

// Helm is a config object for working with helm charts.
type Helm struct {
	chart      v1alpha1.ZarfChart
	chartPath  string
	valuesPath string

	cfg     *types.PackagerConfig
	cluster *cluster.Cluster
	timeout time.Duration
	retries int

	kubeVersion string

	chartOverride   *chart.Chart
	valuesOverrides map[string]any

	settings       *cli.EnvSettings
	actionConfig   *action.Configuration
	variableConfig *variables.VariableConfig
	state          *types.ZarfState
}

// Modifier is a function that modifies the Helm config.
type Modifier func(*Helm)

// New returns a new Helm config struct.
func New(chart v1alpha1.ZarfChart, chartPath string, valuesPath string, mods ...Modifier) *Helm {
	h := &Helm{
		chart:      chart,
		chartPath:  chartPath,
		valuesPath: valuesPath,
		timeout:    config.ZarfDefaultTimeout,
	}

	for _, mod := range mods {
		mod(h)
	}

	return h
}

// NewClusterOnly returns a new Helm config struct geared toward interacting with the cluster (not packages)
func NewClusterOnly(cfg *types.PackagerConfig, variableConfig *variables.VariableConfig, state *types.ZarfState, cluster *cluster.Cluster) *Helm {
	return &Helm{
		cfg:            cfg,
		variableConfig: variableConfig,
		state:          state,
		cluster:        cluster,
		timeout:        config.ZarfDefaultTimeout,
		retries:        config.ZarfDefaultRetries,
	}
}

// NewFromZarfManifest generates a helm chart and config from a given Zarf manifest.
func NewFromZarfManifest(manifest v1alpha1.ZarfManifest, manifestPath, packageName, componentName string, mods ...Modifier) (h *Helm, err error) {
	spinner := message.NewProgressSpinner("Starting helm chart generation %s", manifest.Name)
	defer spinner.Stop()

	// Generate a new chart.
	tmpChart := new(chart.Chart)
	tmpChart.Metadata = new(chart.Metadata)

	// Generate a hashed chart name.
	rawChartName := fmt.Sprintf("raw-%s-%s-%s", packageName, componentName, manifest.Name)
	hasher := sha1.New()
	hasher.Write([]byte(rawChartName))
	tmpChart.Metadata.Name = rawChartName
	sha1ReleaseName := hex.EncodeToString(hasher.Sum(nil))

	// This is fun, increment forward in a semver-way using epoch so helm doesn't cry.
	tmpChart.Metadata.Version = fmt.Sprintf("0.1.%d", config.GetStartTime())
	tmpChart.Metadata.APIVersion = chart.APIVersionV1

	// Add the manifest files so helm does its thing.
	for _, file := range manifest.Files {
		spinner.Updatef("Processing %s", file)
		manifest := path.Join(manifestPath, file)
		data, err := os.ReadFile(manifest)
		if err != nil {
			return h, fmt.Errorf("unable to read manifest file %s: %w", manifest, err)
		}

		// Escape all chars and then wrap in {{ }}.
		txt := strconv.Quote(string(data))
		data = []byte("{{" + txt + "}}")

		tmpChart.Templates = append(tmpChart.Templates, &chart.File{Name: manifest, Data: data})
	}

	// Generate the struct to pass to InstallOrUpgradeChart().
	h = &Helm{
		chart: v1alpha1.ZarfChart{
			Name: tmpChart.Metadata.Name,
			// Preserve the zarf prefix for chart names to match v0.22.x and earlier behavior.
			ReleaseName: fmt.Sprintf("zarf-%s", sha1ReleaseName),
			Version:     tmpChart.Metadata.Version,
			Namespace:   manifest.Namespace,
			NoWait:      manifest.NoWait,
		},
		chartOverride: tmpChart,
		timeout:       config.ZarfDefaultTimeout,
	}

	for _, mod := range mods {
		mod(h)
	}

	spinner.Success()

	return h, nil
}

// WithDeployInfo adds the necessary information to deploy a given chart
func WithDeployInfo(cfg *types.PackagerConfig, variableConfig *variables.VariableConfig, state *types.ZarfState, cluster *cluster.Cluster, valuesOverrides map[string]any, timeout time.Duration, retries int) Modifier {
	return func(h *Helm) {
		h.cfg = cfg
		h.variableConfig = variableConfig
		h.state = state
		h.cluster = cluster
		h.valuesOverrides = valuesOverrides
		h.timeout = timeout
		h.retries = retries
	}
}

// WithKubeVersion sets the Kube version for templating the chart
func WithKubeVersion(kubeVersion string) Modifier {
	return func(h *Helm) {
		h.kubeVersion = kubeVersion
	}
}

// WithVariableConfig sets the variable config for the chart
func WithVariableConfig(variableConfig *variables.VariableConfig) Modifier {
	return func(h *Helm) {
		h.variableConfig = variableConfig
	}
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart v1alpha1.ZarfChart) string {
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}

// StandardValuesName generates a predictable full path for the values file for a helm chart for zarf
func StandardValuesName(destination string, chart v1alpha1.ZarfChart, idx int) string {
	return fmt.Sprintf("%s-%d", StandardName(destination, chart), idx)
}
