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

	// ValuesFiles land at global index 0; TemplatedValuesFiles at global index 1 (len(ValuesFiles) + local index).
	regularPath := StandardValuesName(tmpDir, chart, 0)
	templatedPath := StandardValuesName(tmpDir, chart, 1)

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
