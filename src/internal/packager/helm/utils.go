// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/defenseunicorns/pkg/helpers/v2"
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
func (h *Helm) loadChartFromTarball() (*chart.Chart, error) {
	// Get the path the temporary helm chart tarball
	sourceFile := StandardName(h.chartPath, h.chart) + ".tgz"

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
func (h *Helm) parseChartValues() (chartutil.Values, error) {
	valueOpts := &values.Options{}

	for idx := range h.chart.ValuesFiles {
		path := StandardValuesName(h.valuesPath, h.chart, idx)
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

	return helpers.MergeMapRecursive(chartValues, h.valuesOverrides), nil
}

func (h *Helm) createActionConfig(ctx context.Context, namespace string, spinner *message.Spinner) error {
	// Initialize helm SDK
	actionConfig := new(action.Configuration)
	// Set the settings for the helm SDK
	h.settings = cli.New()

	// Set the namespace for helm
	h.settings.SetNamespace(namespace)

	// Setup K8s connection
	var helmLogger action.DebugLog
	if logger.Enabled(ctx) {
		l := logger.From(ctx)
		helmLogger = slog.NewLogLogger(l.Handler(), slog.LevelDebug).Printf
	} else {
		helmLogger = spinner.Updatef
	}

	err := actionConfig.Init(h.settings.RESTClientGetter(), namespace, "", helmLogger)

	// Set the actionConfig is the received Helm pointer
	h.actionConfig = actionConfig

	return err
}
