// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load_test

import (
	"fmt"
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

func TestChartSourceOverrides(t *testing.T) {
	t.Parallel()
	helm := func(url, version, name string) v1beta1.Chart {
		return v1beta1.Chart{HelmRepository: &v1beta1.HelmRepositorySource{URL: url, Version: version, Name: name}}
	}
	git := func(url, path string, ref v1beta1.GitRef) v1beta1.Chart {
		return v1beta1.Chart{Git: &v1beta1.GitSource{URL: url, Path: path, Ref: ref}}
	}
	oci := func(url string, ref v1beta1.OCIRef) v1beta1.Chart {
		return v1beta1.Chart{OCI: &v1beta1.OCISource{URL: url, Ref: ref}}
	}
	local := func(path string) v1beta1.Chart {
		return v1beta1.Chart{Local: &v1beta1.LocalSource{Path: path}}
	}
	const url = "https://example.com/charts"
	const gitURL = "https://example.com/charts.git"
	const ociURL = "oci://example.com/charts/app"
	tag := v1beta1.GitRef{Tag: "v1"}
	branch := v1beta1.GitRef{Branch: "main"}
	ociTag := v1beta1.OCIRef{Tag: "v1"}
	digest := v1beta1.OCIRef{Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	for _, componentConfig := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			base     v1beta1.Chart
			override v1beta1.Chart
			want     v1beta1.Chart
			middle   *v1beta1.Chart
			wantErr  string
		}{
			{name: "Helm version", base: helm(url, "1.0.0", "upstream"), override: helm("", "2.0.0", ""), want: helm(url, "2.0.0", "upstream")},
			{name: "Helm version with repeated URL", base: helm(url, "1.0.0", "upstream"), override: helm(url, "2.0.0", ""), want: helm(url, "2.0.0", "upstream")},
			{name: "reject Helm URL", base: helm(url, "1.0.0", "upstream"), override: helm(url+"/other", "2.0.0", ""), wantErr: "cannot override helmRepository.url"},
			{name: "reject Helm name", base: helm(url, "1.0.0", "upstream"), override: helm("", "2.0.0", "other"), wantErr: "cannot override helmRepository.name"},
			{name: "reject default Helm name", base: helm(url, "1.0.0", ""), override: helm("", "2.0.0", "other"), wantErr: "cannot override helmRepository.name"},
			{name: "reject Git URL", base: git(gitURL, "charts/app", tag), override: git(gitURL+".other", "", branch), wantErr: "cannot override git.url"},
			{name: "reject Git path", base: git(gitURL, "charts/app", tag), override: git("", "charts/other", branch), wantErr: "cannot override git.path"},
			{name: "reject default Git path", base: git(gitURL, "", tag), override: git("", "charts/other", branch), wantErr: "cannot override git.path"},
			{name: "Git ref replaces tag with branch", base: git(gitURL, "charts/app", tag), override: git("", "", branch), want: git(gitURL, "charts/app", branch)},
			{name: "reject OCI URL", base: oci(ociURL, ociTag), override: oci(ociURL+"-other", digest), wantErr: "cannot override oci.url"},
			{name: "OCI ref replaces tag with digest", base: oci(ociURL, ociTag), override: oci("", digest), want: oci(ociURL, digest)},
			{name: "reject local path", base: local("chart"), override: local("other-chart"), wantErr: "cannot override local.path"},
			{name: "nested Helm versions", base: helm(url, "1.0.0", "upstream"), middle: new(helm("", "1.5.0", "")), override: helm("", "2.0.0", ""), want: helm(url, "2.0.0", "upstream")},
			{name: "reject nested redirect", base: helm(url, "1.0.0", "upstream"), middle: new(helm(url+"/middle", "1.5.0", "")), override: helm("", "2.0.0", ""), wantErr: "cannot override helmRepository.url"},
			{name: "nested omitted local path", base: local("chart"), middle: new(local("")), override: local(""), want: local("base/chart")},
			{name: "Helm and OCI conflict", base: helm(url, "1.0.0", "upstream"), override: oci(ociURL, ociTag), wantErr: `chart "app" must include exactly one source`},
			{name: "Git and Helm conflict", base: git(gitURL, "charts/app", tag), override: helm(url, "1.0.0", ""), wantErr: `chart "app" must include exactly one source`},
			{name: "OCI and local conflict", base: oci(ociURL, ociTag), override: local("chart"), wantErr: `chart "app" must include exactly one source`},
			{name: "conflict in nested import", base: helm(url, "1.0.0", "upstream"), middle: new(oci(ociURL, ociTag)), override: helm("", "2.0.0", ""), wantErr: `chart "app" must include exactly one source`},
		} {
			t.Run(fmt.Sprintf("component=%t/%s", componentConfig, tc.name), func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				tc.base.Name, tc.base.Namespace = "app", "app"
				writeChartComponent(t, filepath.Join(dir, "base", "component.yaml"), v1beta1.ComponentSpec{Charts: []v1beta1.Chart{tc.base}})
				importPath := "base/component.yaml"
				if tc.middle != nil {
					tc.middle.Name = "app"
					writeChartComponent(t, filepath.Join(dir, "middle.yaml"), v1beta1.ComponentSpec{
						Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: importPath}}},
						Charts: []v1beta1.Chart{*tc.middle},
					})
					importPath = "middle.yaml"
				}
				tc.override.Name = "app"
				charts, err := loadChartDefinition(t, dir, componentConfig, v1beta1.ComponentSpec{
					Import: v1beta1.ComponentImport{Local: []v1beta1.ComponentImportLocal{{Path: importPath}}},
					Charts: []v1beta1.Chart{tc.override},
				})
				if tc.wantErr != "" {
					require.ErrorContains(t, err, tc.wantErr)
					return
				}
				require.NoError(t, err)
				tc.want.Name, tc.want.Namespace = "app", "app"
				require.Equal(t, []v1beta1.Chart{tc.want}, charts)
			})
		}
	}
}

func TestChartSourcesMustBeCompleteAfterResolution(t *testing.T) {
	t.Parallel()
	for _, componentConfig := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			chart   v1beta1.Chart
			wantErr string
		}{
			{name: "Helm URL", chart: v1beta1.Chart{HelmRepository: &v1beta1.HelmRepositorySource{Version: "1.0.0"}}, wantErr: "helmRepository.url"},
			{name: "Helm version", chart: v1beta1.Chart{HelmRepository: &v1beta1.HelmRepositorySource{URL: "https://example.com/charts"}}, wantErr: "helmRepository.version"},
			{name: "Git URL", chart: v1beta1.Chart{Git: &v1beta1.GitSource{Ref: v1beta1.GitRef{Tag: "v1"}}}, wantErr: "git.url"},
			{name: "Git ref", chart: v1beta1.Chart{Git: &v1beta1.GitSource{URL: "https://example.com/charts.git"}}, wantErr: "must include git.ref"},
			{name: "OCI URL", chart: v1beta1.Chart{OCI: &v1beta1.OCISource{Ref: v1beta1.OCIRef{Tag: "v1"}}}, wantErr: "oci.url"},
			{name: "OCI ref", chart: v1beta1.Chart{OCI: &v1beta1.OCISource{URL: "oci://example.com/chart"}}, wantErr: "must include oci.ref"},
		} {
			t.Run(fmt.Sprintf("component=%t/%s", componentConfig, tc.name), func(t *testing.T) {
				t.Parallel()
				tc.chart.Name, tc.chart.Namespace = "app", "app"
				_, err := loadChartDefinition(t, t.TempDir(), componentConfig, v1beta1.ComponentSpec{Charts: []v1beta1.Chart{tc.chart}})
				require.ErrorContains(t, err, tc.wantErr)
			})
		}
	}
}

func writeChartComponent(t *testing.T, path string, spec v1beta1.ComponentSpec) {
	t.Helper()
	b, err := goyaml.Marshal(v1beta1.ComponentConfig{
		APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfComponentConfig,
		Metadata: v1beta1.ComponentMetadata{Name: "app"}, Component: spec,
	})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, b, 0o600))
}

func loadChartDefinition(t *testing.T, dir string, componentConfig bool, spec v1beta1.ComponentSpec) ([]v1beta1.Chart, error) {
	t.Helper()
	if componentConfig {
		path := filepath.Join(dir, "component.yaml")
		writeChartComponent(t, path, spec)
		resolved, err := load.ComponentDefinition(t.Context(), path, load.ComponentOptions{})
		return resolved.Component.Charts, err
	}
	b, err := goyaml.Marshal(v1beta1.Package{
		APIVersion: v1beta1.APIVersion, Kind: v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: "app"},
		Components: []v1beta1.Component{{Name: "app", ComponentSpec: spec}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), b, 0o600))
	resolved, err := load.PackageDefinition(t.Context(), dir, load.DefinitionOptions{})
	if err != nil {
		return nil, err
	}
	return convert.PackageToV1beta1(resolved).Components[0].Charts, nil
}
