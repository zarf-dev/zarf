// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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
				Constants: []v1alpha1.Constant{
					{
						Name:    "BAD",
						Pattern: "^good_val$",
						Value:   "bad_val",
					},
				},
			},
			expectedErrs: []string{
				fmt.Errorf(PkgValidateErrConstant, fmt.Errorf("provided value for constant %s does not match pattern %s", "BAD", "^good_val$")).Error(),
				fmt.Sprintf(PkgValidateErrComponentReqDefault, "invalid"),
				fmt.Sprintf(PkgValidateErrChartNameNotUnique, "chart1"),
				fmt.Sprintf(PkgValidateErrManifestNameNotUnique, "manifest1"),
				fmt.Sprintf(PkgValidateErrComponentReqGrouped, "required-in-group"),
				fmt.Sprintf(PkgValidateErrComponentNameNotUnique, "duplicate"),
				fmt.Sprintf(PkgValidateErrGroupOneComponent, "a-group", "required-in-group"),
				fmt.Sprintf(PkgValidateErrGroupMultipleDefaults, "multi-default", "multi-default", "multi-default-2"),
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
				PkgValidateErrInitNoYOLO,
				PkgValidateErrYOLONoOCI,
				PkgValidateErrYOLONoGit,
				PkgValidateErrYOLONoArch,
				PkgValidateErrYOLONoDistro,
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
			expectedErrs: []string{fmt.Sprintf(PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength)},
		},
		{
			name:         "no files or kustomize",
			manifest:     v1alpha1.ZarfManifest{Name: "nothing-there"},
			expectedErrs: []string{fmt.Sprintf(PkgValidateErrManifestFileOrKustomize, "nothing-there")},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateManifest(tt.manifest)
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
				fmt.Sprintf(PkgValidateErrChartName, longName, ZarfMaxChartNameLength),
			},
		},
		{
			name:  "no url, local path, version, or namespace",
			chart: v1alpha1.ZarfChart{Name: "invalid"},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartNamespaceMissing, "invalid"),
				fmt.Sprintf(PkgValidateErrChartURLOrPath, "invalid"),
				fmt.Sprintf(PkgValidateErrChartVersion, "invalid"),
			},
		},
		{
			name:  "both url and local path",
			chart: v1alpha1.ZarfChart{Name: "invalid", Namespace: "whatever", URL: "http://whatever", LocalPath: "wherever", Version: "v1.0.0"},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartURLOrPath, "invalid"),
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
			err := validateChart(tt.chart)
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
							SetVariables: []v1alpha1.Variable{{Name: "VAR"}},
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
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "create")).Error(),
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "deploy")).Error(),
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "remove")).Error(),
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "remove2")).Error(),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateActions(tt.actions)
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
				fmt.Sprintf(PkgValidateErrActionCmdWait, "ls"),
				PkgValidateErrActionClusterNetwork,
			},
		},
		{
			name: "cluster and network both set",
			action: v1alpha1.ZarfComponentAction{
				Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{}, Network: &v1alpha1.ZarfComponentActionWaitNetwork{}},
			},
			expectedErrs: []string{PkgValidateErrActionClusterNetwork},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAction(tt.action)
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}
