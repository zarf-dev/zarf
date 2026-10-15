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
			name: "v1beta1 init kind accepted by v1beta1 branch",
			doc: map[string]any{
				"apiVersion": "zarf.dev/v1beta1",
				"kind":       "ZarfInitConfig",
				"metadata":   map[string]any{"name": "test"},
				"components": []any{map[string]any{"name": "first"}},
			},
			valid: true,
		},
		{
			name: "v1beta1 init service accepted",
			doc: map[string]any{
				"apiVersion": "zarf.dev/v1beta1",
				"kind":       "ZarfInitConfig",
				"metadata":   map[string]any{"name": "test"},
				"components": []any{map[string]any{"name": "registry", "service": "registry"}},
			},
			valid: true,
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

func TestV1Beta1SchemaAllowsChartOverrides(t *testing.T) {
	t.Parallel()
	for _, sourceSchema := range []struct {
		name      string
		schema    []byte
		component bool
	}{
		{name: "package", schema: GetV1Beta1Schema()},
		{name: "combined", schema: getSchema()},
		{name: "component", schema: GetV1Beta1ComponentSchema(), component: true},
	} {
		t.Run(sourceSchema.name, func(t *testing.T) {
			t.Parallel()
			spec := map[string]any{
				"import": map[string]any{"local": []any{map[string]any{"path": "base.yaml"}}},
				"charts": []any{map[string]any{"name": "app", "namespace": "override"}},
			}
			doc := map[string]any{
				"apiVersion": "zarf.dev/v1beta1", "kind": "ZarfPackageConfig",
				"metadata": map[string]any{"name": "app"},
			}
			if sourceSchema.component {
				doc["kind"] = "ZarfComponentConfig"
				doc["component"] = spec
			} else {
				spec["name"] = "app"
				doc["components"] = []any{spec}
			}
			result, err := gojsonschema.Validate(gojsonschema.NewBytesLoader(sourceSchema.schema), gojsonschema.NewGoLoader(doc))
			require.NoError(t, err)
			require.True(t, result.Valid(), result.Errors())
		})
	}
}
