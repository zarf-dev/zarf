// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

func TestGenerateValuesSchema(t *testing.T) {
	t.Parallel()
	setupInspectTests(t)

	for _, kind := range []v1beta1.PackageKind{v1beta1.ZarfPackageConfig, v1beta1.ZarfComponentConfig, "v1alpha1"} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			chart := v1beta1.Chart{
				Name: "app", Namespace: "default",
				Local: &v1beta1.LocalSource{Path: "chart"},
				ValuesFiles: []v1beta1.ValuesFile{
					{Path: "chart-values.yaml", EnableTemplating: true},
					{Path: "literal-values.yaml"},
				},
				Values: []v1beta1.ChartValue{{SourcePath: ".chartPassword", TargetPath: ".password"}},
			}
			values := v1beta1.Values{Files: []string{"values.yaml"}}
			component := v1beta1.ComponentSpec{Charts: []v1beta1.Chart{chart}}
			pkg := v1beta1.Package{
				APIVersion: v1beta1.APIVersion, Kind: kind,
				Metadata: v1beta1.PackageMetadata{Name: "app"}, Values: values,
				Components: []v1beta1.Component{{Name: "app", ComponentSpec: component}},
			}
			var definition any = pkg
			if kind == v1beta1.ZarfComponentConfig {
				definition = v1beta1.ComponentConfig{
					APIVersion: v1beta1.APIVersion, Kind: kind,
					Metadata: v1beta1.ComponentMetadata{Name: "app"}, Values: values, Component: component,
				}
			}
			if kind == "v1alpha1" {
				pkg.Kind = v1beta1.ZarfPackageConfig
				definition = convert.PackageV1beta1ToV1alpha1(pkg)
			}
			contents, err := yaml.Marshal(definition)
			require.NoError(t, err)
			path := filepath.Join(dir, "definition.yaml")
			require.NoError(t, os.WriteFile(path, contents, 0o600))
			files := map[string]string{
				"values.yaml":         "password: secret\n",
				"chart/Chart.yaml":    "apiVersion: v2\nname: app\nversion: 0.1.0\n",
				"chart/values.yaml":   "password: 1\n",
				"chart-values.yaml":   "password: {{ .Values.password }}\n",
				"literal-values.yaml": "literal: '{{ .Values.notDefined }}'\n",
			}
			for name, contents := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600))
			}

			opts := GenerateValuesSchemaOptions{Update: true}
			schema, err := GenerateValuesSchema(t.Context(), path, opts)
			require.NoError(t, err)
			passwordSchema, found, err := value.ExtractJSONSchema(schema, ".chartPassword")
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, "string", passwordSchema["type"])

			updated, err := os.ReadFile(path)
			require.NoError(t, err)
			var probe struct {
				Values v1beta1.Values `json:"values"`
			}
			require.NoError(t, yaml.Unmarshal(updated, &probe))
			require.Equal(t, v1beta1.Values{Files: values.Files, Schema: "values.schema.json"}, probe.Values)
			_, _, err = value.LoadValidatedSchema(dir, filepath.Join(dir, probe.Values.Schema))
			require.NoError(t, err)

			for name, contents := range files {
				actual, err := os.ReadFile(filepath.Join(dir, name))
				require.NoError(t, err)
				require.Equal(t, contents, string(actual))
			}
		})
	}
}

func TestGenerateValuesSchemaComponentWithStaleSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"component.yaml": `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: app
component: {}
values:
  files:
    - values.yaml
  schema: custom.schema.json
`,
		"values.yaml":        "replicas: 1\n",
		"custom.schema.json": `{"type":"object","properties":{"replicas":{"type":"string","description":"Replica count"}}}`,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600))
	}
	schema, err := GenerateValuesSchema(t.Context(), filepath.Join(dir, "component.yaml"), GenerateValuesSchemaOptions{Update: true})
	require.NoError(t, err)
	replicas, found, err := value.ExtractJSONSchema(schema, ".replicas")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "integer", replicas["type"])
	require.Equal(t, "Replica count", replicas["description"])
	written, _, err := value.LoadValidatedSchema(dir, filepath.Join(dir, "custom.schema.json"))
	require.NoError(t, err)
	require.Equal(t, schema, written)
	require.NoFileExists(t, filepath.Join(dir, "values.schema.json"))
}
