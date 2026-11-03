// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

// GenerateChartOverrides generates chart overrides with increasing precedence:
// Zarf variables, mapped Zarf values, then direct overrides.
func GenerateChartOverrides(chart api.Chart, variableConfig *variables.VariableConfig, vals value.Values, overrides map[string]any) (map[string]any, error) {
	chartOverrides := make(value.Values)

	for _, variable := range chart.Variables {
		if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Add leading dot to variable.Path to create a valid value.Path
			path := "." + variable.Path
			if err := chartOverrides.Set(value.Path(path), setVar.Value); err != nil {
				return nil, fmt.Errorf("unable to set value at path %s: %w", path, err)
			}
		}
	}

	// Map ChartValues' Source to Target
	for _, chartValue := range chart.Values {
		if chartValue.SourcePath == "" || chartValue.TargetPath == "" {
			return nil, fmt.Errorf("sourcePath \"%s\" and targetPath \"%s\" must not be empty", chartValue.SourcePath, chartValue.TargetPath)
		}
		if chartValue.SourcePath[0] != '.' {
			return nil, fmt.Errorf("sourcePath \"%s\" must start with a dot", chartValue.SourcePath)
		}
		if chartValue.TargetPath[0] != '.' {
			return nil, fmt.Errorf("targetPath \"%s\" must start with a dot", chartValue.TargetPath)
		}

		// Drop any excluded sub-paths before extracting. Work on a copy so the
		// shared source values are not mutated for other charts.
		sourceValues := vals
		if len(chartValue.ExcludePaths) > 0 {
			sourceValues = vals.DeepCopy()
			for _, excludePath := range chartValue.ExcludePaths {
				if err := sourceValues.Delete(value.Path(excludePath)); err != nil {
					return nil, fmt.Errorf("unable to exclude path %s: %w", excludePath, err)
				}
			}
		}

		// Extract value from source path in values
		sourceValue, err := sourceValues.Extract(value.Path(chartValue.SourcePath))
		if err != nil {
			return nil, fmt.Errorf("unable to extract value source: %w", err)
		}

		// Set value at targetPath in chart overrides
		if err := chartOverrides.Set(value.Path(chartValue.TargetPath), sourceValue); err != nil {
			return nil, fmt.Errorf("unable to map value from %s to %s: %w",
				chartValue.SourcePath, chartValue.TargetPath, err)
		}
	}

	// Merge overrides into chartOverrides (overrides takes precedence)
	chartOverrides.DeepMerge(overrides)
	return chartOverrides, nil
}
