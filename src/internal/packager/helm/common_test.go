// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestParseChartValues(t *testing.T) {
	t.Parallel()

	chart := v1alpha1.ZarfChart{
		Name:    "test",
		Version: "1.0.0",
		// One entry each — length drives index iteration; the string content is not used for path resolution.
		ValuesFiles:          []string{"regular.yaml"},
		TemplatedValuesFiles: []string{"templated.yaml"},
	}

	tmpDir := t.TempDir()

	// ValuesFiles land at StandardValuesName paths; TemplatedValuesFiles at StandardTemplatedValuesName paths.
	regularPath := StandardValuesName(tmpDir, chart, 0)
	templatedPath := StandardTemplatedValuesName(tmpDir, chart, 0)

	require.NoError(t, os.WriteFile(regularPath, []byte("shared: from-regular\nregularOnly: present"), 0o644))
	require.NoError(t, os.WriteFile(templatedPath, []byte("shared: from-templated\ntemplatedOnly: present"), 0o644))

	merged, err := parseChartValues(chart, tmpDir, nil)
	require.NoError(t, err)

	// TemplatedValuesFiles are appended after ValuesFiles in the Helm merge list,
	// so their values take precedence on collision.
	require.Equal(t, "from-templated", merged["shared"])
	require.Equal(t, "present", merged["regularOnly"])
	require.Equal(t, "present", merged["templatedOnly"])
}

func TestStandardTemplatedValuesName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		chart       v1alpha1.ZarfChart
		destination string
		idx         int
		expected    string
	}{
		{
			name:        "versioned chart",
			chart:       v1alpha1.ZarfChart{Name: "my-chart", Version: "1.2.3"},
			destination: "/values",
			idx:         0,
			expected:    "/values/my-chart-1.2.3-templated-0",
		},
		{
			name:        "second templated file",
			chart:       v1alpha1.ZarfChart{Name: "my-chart", Version: "1.2.3"},
			destination: "/values",
			idx:         1,
			expected:    "/values/my-chart-1.2.3-templated-1",
		},
		{
			name:        "no collision with StandardValuesName index",
			chart:       v1alpha1.ZarfChart{Name: "my-chart", Version: "1.2.3"},
			destination: "/values",
			idx:         0,
			// StandardValuesName(0) = "/values/my-chart-1.2.3-0"
			// StandardTemplatedValuesName(0) = "/values/my-chart-1.2.3-templated-0"
			expected: "/values/my-chart-1.2.3-templated-0",
		},
		{
			name:        "chart with no version",
			chart:       v1alpha1.ZarfChart{Name: "my-chart", Version: ""},
			destination: "/values",
			idx:         0,
			expected:    "/values/my-chart-templated-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StandardTemplatedValuesName(tt.destination, tt.chart, tt.idx)
			require.Equal(t, tt.expected, got)
			// Verify no overlap with regular values name at the same index.
			regular := StandardValuesName(tt.destination, tt.chart, tt.idx)
			require.NotEqual(t, regular, got)
		})
	}
}
