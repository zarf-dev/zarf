// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package rules checks Zarf packages and reports any findings or errors
package rules

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestZarfPackageValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		pkg          v1alpha1.ZarfPackage
		expectedErrs []string
	}{
		{
			name: "valid package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "valid-package",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "component1",
					},
				},
			},
			expectedErrs: nil,
		},
		{
			name: "invalid package",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "invalid-package",
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:     "invalid",
						Required: helpers.BoolPtr(true),
						Default:  true,
						Charts: []v1alpha1.ZarfChart{
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
						},
						Manifests: []v1alpha1.ZarfManifest{
							{Name: "manifest1", Files: []string{"file1"}},
							{Name: "manifest1", Files: []string{"file2"}},
						},
					},
					{
						Name:            "required-in-group",
						Required:        helpers.BoolPtr(true),
						DeprecatedGroup: "a-group",
					},
					{
						Name:            "multi-default",
						Default:         true,
						DeprecatedGroup: "multi-default",
					},
					{
						Name:            "multi-default-2",
						Default:         true,
						DeprecatedGroup: "multi-default",
					},
					{
						Name: "duplicate",
					},
					{
						Name: "duplicate",
					},
				},
				Constants: []variables.Constant{
					{
						Name:    "BAD",
						Pattern: "^good_val$",
						Value:   "bad_val",
					},
				},
			},
			expectedErrs: []string{
				fmt.Errorf(lang.PkgValidateErrConstant, fmt.Errorf(lang.PkgValidateErrPkgConstantPattern, "BAD", "^good_val$")).Error(),
				fmt.Sprintf(lang.PkgValidateErrComponentReqDefault, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartNameNotUnique, "chart1"),
				fmt.Sprintf(lang.PkgValidateErrManifestNameNotUnique, "manifest1"),
				fmt.Sprintf(lang.PkgValidateErrComponentReqGrouped, "required-in-group"),
				fmt.Sprintf(lang.PkgValidateErrComponentNameNotUnique, "duplicate"),
				fmt.Sprintf(lang.PkgValidateErrGroupOneComponent, "a-group", "required-in-group"),
				fmt.Sprintf(lang.PkgValidateErrGroupMultipleDefaults, "multi-default", "multi-default", "multi-default-2"),
			},
		},
		{
			name: "invalid yolo",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Metadata: v1alpha1.ZarfMetadata{
					Name: "invalid-yolo",
					YOLO: true,
				},
				Components: []v1alpha1.ZarfComponent{
					{
						Name:   "yolo",
						Images: []string{"an-image"},
						Repos:  []string{"a-repo"},
						Only: v1alpha1.ZarfComponentOnlyTarget{
							Cluster: v1alpha1.ZarfComponentOnlyCluster{
								Architecture: "not-empty",
								Distros:      []string{"not-empty"},
							},
						},
					},
				},
			},
			expectedErrs: []string{
				lang.PkgValidateErrInitNoYOLO,
				lang.PkgValidateErrYOLONoOCI,
				lang.PkgValidateErrYOLONoGit,
				lang.PkgValidateErrYOLONoArch,
				lang.PkgValidateErrYOLONoDistro,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePackage(tt.pkg)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, errs, tt.expectedErrs)
		})
	}
}

func TestValidateManifest(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("a", ZarfMaxChartNameLength+1)
	tests := []struct {
		manifest     v1alpha1.ZarfManifest
		expectedErrs []string
		name         string
	}{
		{
			name:         "valid",
			manifest:     v1alpha1.ZarfManifest{Name: "valid", Files: []string{"a-file"}},
			expectedErrs: nil,
		},
		{
			name:         "long name",
			manifest:     v1alpha1.ZarfManifest{Name: longName, Files: []string{"a-file"}},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength)},
		},
		{
			name:         "no files or kustomize",
			manifest:     v1alpha1.ZarfManifest{Name: "nothing-there"},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestFileOrKustomize, "nothing-there")},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateManifest(tt.manifest)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, errs, tt.expectedErrs)
		})
	}
}

func TestValidateReleaseName(t *testing.T) {
	tests := []struct {
		name           string
		chartName      string
		releaseName    string
		expectError    bool
		errorSubstring string
	}{
		{
			name:        "valid releaseName with hyphens",
			chartName:   "chart",
			releaseName: "valid-release-hyphenated",
			expectError: false,
		},
		{
			name:        "valid releaseName with numbers",
			chartName:   "chart",
			releaseName: "valid-0470",
			expectError: false,
		},
		{
			name:           "invalid releaseName with periods",
			chartName:      "chart",
			releaseName:    "namedwithperiods-a.b.c",
			expectError:    true,
			errorSubstring: "invalid release name 'namedwithperiods-a.b.c'",
		},
		{
			name:        "empty releaseName, valid chartName",
			chartName:   "valid-chart",
			releaseName: "",
			expectError: false,
		},
		{
			name:           "empty releaseName and chartName",
			chartName:      "",
			releaseName:    "",
			expectError:    true,
			errorSubstring: errChartReleaseNameEmpty,
		},
		{
			name:           "empty releaseName, invalid chartName",
			chartName:      "invalid_chart!",
			releaseName:    "",
			expectError:    true,
			errorSubstring: "invalid release name 'invalid_chart!'",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateReleaseName(tt.chartName, tt.releaseName)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorSubstring)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateChart(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("a", ZarfMaxChartNameLength+1)
	tests := []struct {
		name         string
		chart        v1alpha1.ZarfChart
		expectedErrs []string
		partialMatch bool
	}{
		{
			name:         "valid",
			chart:        v1alpha1.ZarfChart{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0", ReleaseName: "this-is-valid"},
			expectedErrs: nil,
		},
		{
			name:  "long name",
			chart: v1alpha1.ZarfChart{Name: longName, Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartName, longName, ZarfMaxChartNameLength),
			},
		},
		{
			name:  "no url, local path, version, or namespace",
			chart: v1alpha1.ZarfChart{Name: "invalid"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartNamespaceMissing, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartVersion, "invalid"),
			},
		},
		{
			name:  "both url and local path",
			chart: v1alpha1.ZarfChart{Name: "invalid", Namespace: "whatever", URL: "http://whatever", LocalPath: "wherever", Version: "v1.0.0"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
			},
		},
		{
			name:         "invalid releaseName",
			chart:        v1alpha1.ZarfChart{ReleaseName: "namedwithperiods-0.47.0", Name: "releaseName", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: []string{"invalid release name 'namedwithperiods-0.47.0'"},
			partialMatch: true,
		},
		{
			name:         "missing releaseName fallsback to name",
			chart:        v1alpha1.ZarfChart{Name: "chart3", Namespace: "namespace", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: nil,
		},
		{
			name:         "missing name and releaseName",
			chart:        v1alpha1.ZarfChart{Namespace: "namespace", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: []string{errChartReleaseNameEmpty},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateChart(tt.chart)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			errString := err.Error()
			if tt.partialMatch {
				for _, expectedErr := range tt.expectedErrs {
					require.Contains(t, errString, expectedErr)
				}
			} else {
				errs := strings.Split(errString, "\n")
				require.ElementsMatch(t, tt.expectedErrs, errs)
			}
		})
	}
}

func TestValidateComponentActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		actions      v1alpha1.ZarfComponentActions
		expectedErrs []string
	}{
		{
			name: "valid actions",
			actions: v1alpha1.ZarfComponentActions{
				OnCreate: v1alpha1.ZarfComponentActionSet{
					Before: []v1alpha1.ZarfComponentAction{
						{
							Cmd: "echo 'onCreate before valid'",
						},
					},
				},
				OnDeploy: v1alpha1.ZarfComponentActionSet{
					Before: []v1alpha1.ZarfComponentAction{
						{
							Cmd: "echo 'onDeploy before valid'",
						},
					},
				},
			},
			expectedErrs: nil,
		},
		{
			name: "setVariables in onCreate",
			actions: v1alpha1.ZarfComponentActions{
				OnCreate: v1alpha1.ZarfComponentActionSet{
					Before: []v1alpha1.ZarfComponentAction{
						{
							Cmd:          "echo 'invalid setVariable'",
							SetVariables: []variables.Variable{{Name: "VAR"}},
						},
					},
				},
			},
			expectedErrs: []string{"cannot contain setVariables outside of onDeploy in actions"},
		},
		{
			name: "invalid onCreate action",
			actions: v1alpha1.ZarfComponentActions{
				OnCreate: v1alpha1.ZarfComponentActionSet{
					Before: []v1alpha1.ZarfComponentAction{
						{
							Cmd:  "create",
							Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}},
						},
					},
				},
				OnDeploy: v1alpha1.ZarfComponentActionSet{
					After: []v1alpha1.ZarfComponentAction{
						{
							Cmd:  "deploy",
							Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}},
						},
					},
				},
				OnRemove: v1alpha1.ZarfComponentActionSet{
					OnSuccess: []v1alpha1.ZarfComponentAction{
						{
							Cmd:  "remove",
							Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}},
						},
					},
					OnFailure: []v1alpha1.ZarfComponentAction{
						{
							Cmd:  "remove2",
							Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}},
						},
					},
				},
			},
			expectedErrs: []string{
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "create")).Error(),
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "deploy")).Error(),
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "remove")).Error(),
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "remove2")).Error(),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateActions(tt.actions)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}

func TestValidateComponentAction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		action       v1alpha1.ZarfComponentAction
		expectedErrs []string
	}{
		{
			name:   "valid action no conditions",
			action: v1alpha1.ZarfComponentAction{},
		},
		{
			name: "cmd and wait both set, nothing in wait",
			action: v1alpha1.ZarfComponentAction{
				Cmd:  "ls",
				Wait: &v1alpha1.ZarfComponentActionWait{},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrActionCmdWait, "ls"),
				lang.PkgValidateErrActionClusterNetwork,
			},
		},
		{
			name: "cluster and network both set",
			action: v1alpha1.ZarfComponentAction{
				Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}, Network: &v1alpha1.ZarfComponentActionWaitNetwork{}},
			},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrActionClusterNetwork)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateAction(tt.action)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}

func TestValidateZarfComponent(t *testing.T) {
	t.Parallel()
	absPath, err := filepath.Abs("abs")
	require.NoError(t, err)
	tests := []struct {
		component    v1alpha1.ZarfComponent
		expectedErrs []string
		name         string
	}{
		{
			name: "valid path",
			component: v1alpha1.ZarfComponent{
				Name: "component1",
				Import: v1alpha1.ZarfComponentImport{
					Path: "relative/path",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "valid URL",
			component: v1alpha1.ZarfComponent{
				Name: "component2",
				Import: v1alpha1.ZarfComponentImport{
					URL: "oci://example.com/package:v0.0.1",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "neither path nor URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "neither",
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "neither", "neither a path nor a URL was provided"),
			},
		},
		{
			name: "both path and URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "both",
				Import: v1alpha1.ZarfComponentImport{
					Path: "relative/path",
					URL:  "https://example.com",
				},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "both", "both a path and a URL were provided"),
			},
		},
		{
			name: "absolute path provided",
			component: v1alpha1.ZarfComponent{
				Name: "abs-path",
				Import: v1alpha1.ZarfComponentImport{
					Path: absPath,
				},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "abs-path", "path cannot be an absolute path"),
			},
		},
		{
			name: "invalid URL provided",
			component: v1alpha1.ZarfComponent{
				Name: "bad-url",
				Import: v1alpha1.ZarfComponentImport{
					URL: "https://example.com",
				},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "bad-url", "URL is not a valid OCI URL"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateComponent(tt.component)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}
