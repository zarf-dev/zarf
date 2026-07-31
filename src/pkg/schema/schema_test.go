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

func TestV1Beta1SourceOneOfSchema(t *testing.T) {
	t.Parallel()

	baseDoc := func(component map[string]any) map[string]any {
		return map[string]any{
			"apiVersion": "zarf.dev/v1beta1",
			"kind":       "ZarfPackageConfig",
			"metadata":   map[string]any{"name": "test"},
			"components": []any{component},
		}
	}
	chartComponent := func(chart map[string]any) map[string]any {
		return map[string]any{
			"name":   "component",
			"charts": []any{chart},
		}
	}

	tests := []struct {
		name  string
		doc   map[string]any
		valid bool
	}{
		{
			name: "chart has exactly one source",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"local": map[string]any{
					"path": "chart",
				},
			})),
			valid: true,
		},
		{
			name: "chart rejects multiple sources",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"local": map[string]any{
					"path": "chart",
				},
				"oci": map[string]any{
					"url": "oci://example.com/chart",
					"ref": map[string]any{"tag": "1.0.0"},
				},
			})),
			valid: false,
		},
		{
			name: "oci ref rejects tag and digest",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"oci": map[string]any{
					"url": "oci://example.com/chart",
					"ref": map[string]any{
						"tag":    "1.0.0",
						"digest": "sha256:abcdef",
					},
				},
			})),
			valid: false,
		},
		{
			name: "git chart ref rejects tag and branch",
			doc: baseDoc(chartComponent(map[string]any{
				"name": "chart",
				"git": map[string]any{
					"url": "https://example.com/repo.git",
					"ref": map[string]any{
						"tag":    "1.0.0",
						"branch": "main",
					},
				},
			})),
			valid: false,
		},
		{
			name: "git repository ref rejects branch and commit",
			doc: baseDoc(map[string]any{
				"name": "component",
				"repositories": []any{
					map[string]any{
						"url": "https://example.com/repo.git",
						"ref": map[string]any{
							"branch": "main",
							"commit": "abc123",
						},
					},
				},
			}),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := gojsonschema.Validate(
				gojsonschema.NewBytesLoader(GetV1Beta1Schema()),
				gojsonschema.NewGoLoader(tt.doc),
			)
			require.NoError(t, err)
			require.Equal(t, tt.valid, result.Valid(), result.Errors())
		})
	}
}
