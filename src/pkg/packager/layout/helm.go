// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"context"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"

	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
)

// LoadChartData loads a Zarf chart and merges its values, returning the Helm
// SDK chart and merged values. Intended for tooling that needs Zarf's chart
// values-merge behavior without shelling out to the `zarf` CLI.
func LoadChartData(chart v1alpha1.ZarfChart, chartPath string, valuesPath string, valuesOverrides map[string]any) (*chartv2.Chart, common.Values, error) {
	return helm.LoadChartData(chart, chartPath, valuesPath, valuesOverrides)
}

// TemplateChart renders a chart with Zarf variable/template substitution
// applied. Intended for tooling that needs Zarf's templating behavior without
// shelling out to the `zarf` CLI.
func TemplateChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chartv2.Chart, values common.Values, kubeVersion string, variableConfig *variables.VariableConfig, isInteractive bool, remoteOptions types.RemoteOptions) (string, error) {
	return helm.TemplateChart(ctx, zarfChart, chart, values, kubeVersion, variableConfig, isInteractive, remoteOptions)
}
