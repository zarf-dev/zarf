// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"fmt"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/variables"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// TemplateChart generates a helm template from a given chart.
func TemplateChart(ctx context.Context, chart v1alpha1.ZarfChart, kubeVersion string, chartPath string, variableConfig *variables.VariableConfig) (manifest string, chartValues chartutil.Values, err error) {
	if variableConfig == nil {
		variableConfig = template.GetZarfVariableConfig(ctx)
	}
	l := logger.From(ctx)
	spinner := message.NewProgressSpinner("Templating helm chart %s", chart.Name)
	defer spinner.Stop()
	l.Debug("templating helm chart", "name", chart.Name)

	actionCfg, err := createActionConfig(ctx, chart.Namespace)
	if err != nil {
		return "", nil, err
	}

	// Bind the helm action.
	client := action.NewInstall(actionCfg)

	client.DryRun = true
	client.Replace = true // Skip the name check.
	client.ClientOnly = true
	client.IncludeCRDs = true
	// TODO: Further research this with regular/OCI charts
	client.Verify = false
	client.InsecureSkipTLSverify = config.CommonOptions.InsecureSkipTLSVerify
	if kubeVersion != "" {
		parsedKubeVersion, err := chartutil.ParseKubeVersion(kubeVersion)
		if err != nil {
			return "", nil, fmt.Errorf("invalid kube version %s: %w", kubeVersion, err)
		}
		client.KubeVersion = parsedKubeVersion
	}
	client.ReleaseName = chart.ReleaseName

	// If no release name is specified, use the chart name.
	if client.ReleaseName == "" {
		client.ReleaseName = chart.Name
	}

	// Namespace must be specified.
	client.Namespace = chart.Namespace

	loadedChart, chartValues, err := loadChartData(chart, chartPath, nil, nil)
	if err != nil {
		return "", nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	client.PostRenderer, err = newTemplateRenderer(chartPath, actionCfg, variableConfig)
	if err != nil {
		return "", nil, fmt.Errorf("unable to create helm renderer: %w", err)
	}

	// Perform the loadedChart installation.
	templatedChart, err := client.RunWithContext(ctx, loadedChart, chartValues)
	if err != nil {
		return "", nil, fmt.Errorf("error generating helm chart template: %w", err)
	}

	manifest = templatedChart.Manifest

	for _, hook := range templatedChart.Hooks {
		manifest += fmt.Sprintf("\n---\n%s", hook.Manifest)
	}

	spinner.Success()

	return manifest, chartValues, nil
}
