// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

// ChartFromZarfManifest generates a helm chart and config from a given Zarf manifest.
func ChartFromZarfManifest(manifest v1alpha1.ZarfManifest, manifestPath, packageName, componentName string) (v1alpha1.ZarfChart, *chart.Chart, error) {
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
		manifest := path.Join(manifestPath, file)
		data, err := os.ReadFile(manifest)
		if err != nil {
			return v1alpha1.ZarfChart{}, nil, fmt.Errorf("unable to read manifest file %s: %w", manifest, err)
		}

		// Escape all chars and then wrap in {{ }}.
		txt := strconv.Quote(string(data))
		data = []byte("{{" + txt + "}}")

		tmpChart.Templates = append(tmpChart.Templates, &chart.File{Name: manifest, Data: data})
	}

	// Generate the struct to pass to InstallOrUpgradeChart().
	chart := v1alpha1.ZarfChart{
		Name: tmpChart.Metadata.Name,
		// Preserve the zarf prefix for chart names to match v0.22.x and earlier behavior.
		ReleaseName: fmt.Sprintf("zarf-%s", sha1ReleaseName),
		Version:     tmpChart.Metadata.Version,
		Namespace:   manifest.Namespace,
		NoWait:      manifest.NoWait,
	}

	return chart, tmpChart, nil
}

// StandardName generates a predictable full path for a helm chart for Zarf.
func StandardName(destination string, chart v1alpha1.ZarfChart) string {
	if chart.Version == "" {
		return filepath.Join(destination, chart.Name)
	}
	return filepath.Join(destination, chart.Name+"-"+chart.Version)
}

// StandardValuesName generates a predictable full path for the values file for a helm chart for zarf
func StandardValuesName(destination string, chart v1alpha1.ZarfChart, idx int) string {
	return fmt.Sprintf("%s-%d", StandardName(destination, chart), idx)
}

// loadChartFromTarball returns a helm chart from a tarball.
func loadChartFromTarball(chart v1alpha1.ZarfChart, chartPath string) (*chart.Chart, error) {
	// Get the path the temporary helm chart tarball
	sourceFile := StandardName(chartPath, chart) + ".tgz"

	// Load the loadedChart tarball
	loadedChart, err := loader.Load(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("unable to load helm chart archive: %w", err)
	}

	if err = loadedChart.Validate(); err != nil {
		return nil, fmt.Errorf("unable to validate loaded helm chart: %w", err)
	}

	return loadedChart, nil
}

// parseChartValues reads the context of the chart values into an interface if it exists.
func parseChartValues(chart v1alpha1.ZarfChart, valuesPath string, valuesOverrides map[string]any) (chartutil.Values, error) {
	valueOpts := &values.Options{}

	for idx := range chart.ValuesFiles {
		path := StandardValuesName(valuesPath, chart, idx)
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, path)
	}

	httpProvider := getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	}

	providers := getter.Providers{httpProvider}
	chartValues, err := valueOpts.MergeValues(providers)
	if err != nil {
		return chartValues, err
	}

	return helpers.MergeMapRecursive(chartValues, valuesOverrides), nil
}

func createActionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	// Set the settings for the helm SDK
	settings := cli.New()
	settings.SetNamespace(namespace)
	l := logger.From(ctx)
	helmLogger := slog.NewLogLogger(l.Handler(), slog.LevelDebug).Printf
	err := actionConfig.Init(settings.RESTClientGetter(), namespace, "", helmLogger)
	if err != nil {
		return nil, fmt.Errorf("could not get Helm action configuration: %w", err)
	}
	return actionConfig, err
}
