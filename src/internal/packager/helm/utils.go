// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"

	"helm.sh/helm/v3/pkg/chart/loader"
)

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

func (h *Helm) createActionConfig(ctx context.Context, namespace string, spinner *message.Spinner) error {
	// Initialize helm SDK
	actionConfig := new(action.Configuration)
	// Set the settings for the helm SDK
	h.settings = cli.New()

	// Set the namespace for helm
	h.settings.SetNamespace(namespace)

	// Setup K8s connection
	helmLogger := spinner.Updatef
	if logger.Enabled(ctx) {
		l := logger.From(ctx)
		helmLogger = slog.NewLogLogger(l.Handler(), slog.LevelDebug).Printf
	}
	err := actionConfig.Init(h.settings.RESTClientGetter(), namespace, "", helmLogger)

	// Set the actionConfig is the received Helm pointer
	h.actionConfig = actionConfig

	return err
}
