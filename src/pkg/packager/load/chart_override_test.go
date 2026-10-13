// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load_test

import (
	"os"
	"path/filepath"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
)

// FIXME: probably simplify this
func TestChartOverridesInheritSource(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"package", "component config"} {
		for _, field := range []string{"namespace", "releaseName", "valuesFiles", "values", "nested import"} {
			t.Run(kind+"/"+field, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				writeConfig := func(path string, config any) {
					t.Helper()
					b, err := goyaml.Marshal(config)
					require.NoError(t, err)
					require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
					require.NoError(t, os.WriteFile(path, b, 0o600))
				}

				baseChart := v1beta1.Chart{
					Name: "app", Namespace: "original", ReleaseName: "original",
					Local:       &v1beta1.LocalSource{Path: "app-chart"},
					ValuesFiles: []v1beta1.ValuesFile{{Path: "base-values.yaml"}},
					Values:      []v1beta1.ChartValue{{SourcePath: ".base", TargetPath: ".base"}},
				}
				writeConfig(filepath.Join(dir, "base", "component.yaml"), v1beta1.ComponentConfig{
					APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
					Metadata:  v1beta1.ComponentMetadata{Name: "base"},
					Component: v1beta1.ComponentSpec{Charts: []v1beta1.Chart{baseChart}},
				})

				expected := baseChart
				expected.Local = &v1beta1.LocalSource{Path: "base/app-chart"}
				expected.ValuesFiles = []v1beta1.ValuesFile{{Path: "base/base-values.yaml"}}
				override := v1beta1.Chart{Name: "app"}
				importPath := "base/component.yaml"
				switch field {
				case "namespace":
					override.Namespace = "overridden"
					expected.Namespace = "overridden"
				case "releaseName":
					override.ReleaseName = "overridden"
					expected.ReleaseName = "overridden"
				case "valuesFiles":
					override.ValuesFiles = []v1beta1.ValuesFile{{Path: "override.yaml", EnableTemplating: true}}
					expected.ValuesFiles = append(expected.ValuesFiles, override.ValuesFiles...)
				case "values":
					override.Values = []v1beta1.ChartValue{{SourcePath: ".override", TargetPath: ".override"}}
					expected.Values = append(expected.Values, override.Values...)
				case "nested import":
					importPath = "middle.yaml"
					writeConfig(filepath.Join(dir, importPath), v1beta1.ComponentConfig{
						APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
						Metadata: v1beta1.ComponentMetadata{Name: "middle"},
						Component: v1beta1.ComponentSpec{
							Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: "base/component.yaml"}}},
							Charts: []v1beta1.Chart{{Name: "app", Namespace: "middle"}},
						},
					})
					override.ReleaseName = "overridden"
					expected.Namespace = "middle"
					expected.ReleaseName = "overridden"
				}
				spec := v1beta1.ComponentSpec{
					Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: importPath}}},
					Charts: []v1beta1.Chart{override},
				}

				var charts []v1beta1.Chart
				if kind == "package" {
					writeConfig(filepath.Join(dir, "zarf.yaml"), v1beta1.Package{
						APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfPackageConfig,
						Metadata:   v1beta1.PackageMetadata{Name: "overrides"},
						Components: []v1beta1.Component{{Name: "app", ComponentSpec: spec}},
					})
					resolved, err := load.PackageDefinition(t.Context(), dir, load.DefinitionOptions{})
					require.NoError(t, err)
					charts = convert.PackageToV1beta1(resolved).Components[0].Charts
				} else {
					path := filepath.Join(dir, "component.yaml")
					writeConfig(path, v1beta1.ComponentConfig{
						APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
						Metadata:  v1beta1.ComponentMetadata{Name: "overrides"},
						Component: spec,
					})
					resolved, err := load.ComponentDefinition(t.Context(), path, load.ComponentOptions{})
					require.NoError(t, err)
					charts = resolved.Component.Charts
				}
				require.Equal(t, []v1beta1.Chart{expected}, charts)
			})
		}
	}
}
