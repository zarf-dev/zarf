// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"fmt"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// testChartPaths is an in-package ChartPaths implementation for tests.
type testChartPaths struct {
	chartsDir string
	valuesDir string
}

func (p testChartPaths) archiveName(chart v1alpha1.ZarfChart) string {
	if chart.Version == "" {
		return chart.Name
	}
	return chart.Name + "-" + chart.Version
}

func (p testChartPaths) Archive(chart v1alpha1.ZarfChart) string {
	return filepath.Join(p.chartsDir, p.archiveName(chart)) + ".tgz"
}

func (p testChartPaths) ValuesFile(chart v1alpha1.ZarfChart, idx int) string {
	return filepath.Join(p.valuesDir, fmt.Sprintf("%s-%d", p.archiveName(chart), idx))
}
