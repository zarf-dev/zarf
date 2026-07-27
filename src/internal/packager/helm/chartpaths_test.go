// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"fmt"
	"path/filepath"
)

// testChartPaths is an in-package ChartPaths implementation for tests.
type testChartPaths struct {
	chartsDir string
	valuesDir string
}

func (p testChartPaths) archiveName(name, version string) string {
	if version == "" {
		return name
	}
	return name + "-" + version
}

func (p testChartPaths) Archive(name, version string) string {
	return filepath.Join(p.chartsDir, p.archiveName(name, version)) + ".tgz"
}

func (p testChartPaths) ValuesFile(name, version string, idx int) string {
	return filepath.Join(p.valuesDir, fmt.Sprintf("%s-%d", p.archiveName(name, version), idx))
}
