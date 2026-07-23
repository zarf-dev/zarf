// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestCombinedSchemaSelectsVersionByAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		doc   map[string]any
		valid bool
	}{
		{
			name: "v1alpha1 without apiVersion",
			doc: map[string]any{
				"kind":       "ZarfPackageConfig",
				"metadata":   map[string]any{"name": "test"},
				"components": []any{map[string]any{"name": "first"}},
			},
			valid: true,
		},
		{
			name: "v1beta1 with apiVersion",
			doc: map[string]any{
				"apiVersion": "zarf.dev/v1beta1",
				"kind":       "ZarfPackageConfig",
				"metadata":   map[string]any{"name": "test"},
				"components": []any{map[string]any{"name": "first"}},
			},
			valid: true,
		},
		{
			name: "v1beta1 init kind rejected by v1beta1 branch",
			doc: map[string]any{
				"apiVersion": "zarf.dev/v1beta1",
				"kind":       "ZarfInitConfig",
				"metadata":   map[string]any{"name": "test"},
				"components": []any{map[string]any{"name": "first"}},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := gojsonschema.Validate(
				gojsonschema.NewBytesLoader(getSchema()),
				gojsonschema.NewGoLoader(tt.doc),
			)
			require.NoError(t, err)
			require.Equal(t, tt.valid, result.Valid(), result.Errors())
		})
	}
}
