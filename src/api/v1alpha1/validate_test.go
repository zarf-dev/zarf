// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 contains all the structs for the v1alpha1 ZarfPackageConfig
package v1alpha1

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestZarfPackageValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		pkg          ZarfPackage
		expectedErrs []string
	}{
		{
			name: "valid package",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
					},
				},
			},
			expectedErrs: nil,
		},
		{
			name: "invalid package",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "invalid-package",
				},
				Components: []ZarfComponent{
					{
						Name:     "invalid",
						Required: helpers.BoolPtr(true),
						Default:  true,
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
						},
						Manifests: []ZarfManifest{
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
			pkg: ZarfPackage{
				Kind: ZarfInitConfig,
				Metadata: ZarfMetadata{
					Name: "invalid-yolo",
					YOLO: true,
				},
				Components: []ZarfComponent{
					{
						Name:   "yolo",
						Images: []string{"an-image"},
						Repos:  []string{"a-repo"},
						Only: ZarfComponentOnlyTarget{
							Cluster: ZarfComponentOnlyCluster{
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
			err := tt.pkg.Validate()
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
		manifest     ZarfManifest
		expectedErrs []string
		name         string
	}{
		{
			name:         "valid",
			manifest:     ZarfManifest{Name: "valid", Files: []string{"a-file"}},
			expectedErrs: nil,
		},
		{
			name:         "long name",
			manifest:     ZarfManifest{Name: longName, Files: []string{"a-file"}},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength)},
		},
		{
			name:         "no files or kustomize",
			manifest:     ZarfManifest{Name: "nothing-there"},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestFileOrKustomize, "nothing-there")},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.manifest.Validate()
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, errs, tt.expectedErrs)
		})
	}
}

func TestValidateChart(t *testing.T) {
	t.Parallel()
	longName := strings.Repeat("a", ZarfMaxChartNameLength+1)
	tests := []struct {
		chart        ZarfChart
		expectedErrs []string
		name         string
	}{
		{
			name:         "valid",
			chart:        ZarfChart{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: nil,
		},
		{
			name:  "long name",
			chart: ZarfChart{Name: longName, Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartName, longName, ZarfMaxChartNameLength),
			},
		},
		{
			name:  "no url, local path, version, or namespace",
			chart: ZarfChart{Name: "invalid"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartNamespaceMissing, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartVersion, "invalid"),
			},
		},
		{
			name:  "both url and local path",
			chart: ZarfChart{Name: "invalid", Namespace: "whatever", URL: "http://whatever", LocalPath: "wherever", Version: "v1.0.0"},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.chart.Validate()
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}

func TestValidateComponentActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		actions      ZarfComponentActions
		expectedErrs []string
	}{
		{
			name: "valid actions",
			actions: ZarfComponentActions{
				OnCreate: ZarfComponentActionSet{
					Before: []ZarfComponentAction{
						{
							Cmd: "echo 'onCreate before valid'",
						},
					},
				},
				OnDeploy: ZarfComponentActionSet{
					Before: []ZarfComponentAction{
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
			actions: ZarfComponentActions{
				OnCreate: ZarfComponentActionSet{
					Before: []ZarfComponentAction{
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
			actions: ZarfComponentActions{
				OnCreate: ZarfComponentActionSet{
					Before: []ZarfComponentAction{
						{
							Cmd:  "create",
							Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}},
						},
					},
				},
				OnDeploy: ZarfComponentActionSet{
					After: []ZarfComponentAction{
						{
							Cmd:  "deploy",
							Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}},
						},
					},
				},
				OnRemove: ZarfComponentActionSet{
					OnSuccess: []ZarfComponentAction{
						{
							Cmd:  "remove",
							Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}},
						},
					},
					OnFailure: []ZarfComponentAction{
						{
							Cmd:  "remove2",
							Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}},
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
			err := tt.actions.validate()
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
		action       ZarfComponentAction
		expectedErrs []string
	}{
		{
			name:   "valid action no conditions",
			action: ZarfComponentAction{},
		},
		{
			name: "cmd and wait both set, nothing in wait",
			action: ZarfComponentAction{
				Cmd:  "ls",
				Wait: &ZarfComponentActionWait{},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrActionCmdWait, "ls"),
				lang.PkgValidateErrActionClusterNetwork,
			},
		},
		{
			name: "cluster and network both set",
			action: ZarfComponentAction{
				Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}, Network: &ZarfComponentActionWaitNetwork{}},
			},
			expectedErrs: []string{fmt.Sprintf(lang.PkgValidateErrActionClusterNetwork)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.action.Validate()
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
		component    ZarfComponent
		expectedErrs []string
		name         string
	}{
		{
			name: "valid path",
			component: ZarfComponent{
				Name: "component1",
				Import: ZarfComponentImport{
					Path: "relative/path",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "valid URL",
			component: ZarfComponent{
				Name: "component2",
				Import: ZarfComponentImport{
					URL: "oci://example.com/package:v0.0.1",
				},
			},
			expectedErrs: nil,
		},
		{
			name: "neither path nor URL provided",
			component: ZarfComponent{
				Name: "neither",
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "neither", "neither a path nor a URL was provided"),
			},
		},
		{
			name: "both path and URL provided",
			component: ZarfComponent{
				Name: "both",
				Import: ZarfComponentImport{
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
			component: ZarfComponent{
				Name: "abs-path",
				Import: ZarfComponentImport{
					Path: absPath,
				},
			},
			expectedErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "abs-path", "path cannot be an absolute path"),
			},
		},
		{
			name: "invalid URL provided",
			component: ZarfComponent{
				Name: "bad-url",
				Import: ZarfComponentImport{
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
			err := tt.component.Validate()
			if tt.expectedErrs == nil {
				require.NoError(t, err)
				return
			}
			errs := strings.Split(err.Error(), "\n")
			require.ElementsMatch(t, tt.expectedErrs, errs)
		})
	}
}
