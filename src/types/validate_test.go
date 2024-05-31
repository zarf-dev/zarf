// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/stretchr/testify/require"
)

func TestZarfPackageValidate(t *testing.T) {
	tests := []struct {
		name     string
		pkg      ZarfPackage
		wantErrs []string
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
			wantErrs: nil,
		},
		{
			name: "no components",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "empty-components",
				},
				Components: []ZarfComponent{},
			},
			wantErrs: []string{"package must have at least 1 component"},
		},
		{
			name: "invalid package",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "-invalid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "-invalid",
						Only: ZarfComponentOnlyTarget{
							LocalOS: "unsupportedOS",
						},
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
						Name: "duplicate",
					},
					{
						Name: "duplicate",
					},
				},
				Variables: []variables.InteractiveVariable{
					{
						Variable: variables.Variable{Name: "not_uppercase"},
					},
				},
				Constants: []variables.Constant{
					{
						Name: "not_uppercase",
					},
					{
						Name:    "BAD",
						Pattern: "^good_val$",
						Value:   "bad_val",
					},
				},
			},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrPkgName, "-invalid-package"),
				fmt.Errorf(lang.PkgValidateErrVariable, fmt.Errorf(lang.PkgValidateMustBeUppercase, "not_uppercase")).Error(),
				fmt.Errorf(lang.PkgValidateErrConstant, fmt.Errorf(lang.PkgValidateErrPkgConstantName, "not_uppercase")).Error(),
				fmt.Errorf(lang.PkgValidateErrConstant, fmt.Errorf(lang.PkgValidateErrPkgConstantPattern, "BAD", "^good_val$")).Error(),
				fmt.Sprintf(lang.PkgValidateErrComponentName, "-invalid"),
				fmt.Sprintf(lang.PkgValidateErrComponentLocalOS, "-invalid", "unsupportedOS", supportedOS),
				fmt.Sprintf(lang.PkgValidateErrComponentReqDefault, "-invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartNameNotUnique, "chart1"),
				fmt.Sprintf(lang.PkgValidateErrManifestNameNotUnique, "manifest1"),
				fmt.Sprintf(lang.PkgValidateErrComponentReqGrouped, "required-in-group"),
				fmt.Sprintf(lang.PkgValidateErrComponentNameNotUnique, "duplicate"),
				fmt.Sprintf(lang.PkgValidateErrGroupOneComponent, "a-group", "required-in-group"),
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
			wantErrs: []string{
				lang.PkgValidateErrInitNoYOLO,
				lang.PkgValidateErrYOLONoOCI,
				lang.PkgValidateErrYOLONoGit,
				lang.PkgValidateErrYOLONoArch,
				lang.PkgValidateErrYOLONoDistro,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pkg.Validate()
			if tt.wantErrs == nil {
				require.NoError(t, err)
				return
			}
			for _, wantErr := range tt.wantErrs {
				require.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestValidateManifest(t *testing.T) {
	longName := ""
	for range ZarfMaxChartNameLength + 1 {
		longName += "a"
	}
	tests := []struct {
		manifest ZarfManifest
		wantErrs []string
		name     string
	}{
		{
			name:     "valid",
			manifest: ZarfManifest{Name: "valid", Files: []string{"a-file"}},
			wantErrs: nil,
		},
		{
			name:     "empty name",
			manifest: ZarfManifest{Name: ""},
			wantErrs: []string{lang.PkgValidateErrManifestNameMissing},
		},
		{
			name:     "long name",
			manifest: ZarfManifest{Name: longName, Files: []string{"a-file"}},
			wantErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength)},
		},
		{
			name:     "no files or kustomize",
			manifest: ZarfManifest{Name: "nothing-there"},
			wantErrs: []string{fmt.Sprintf(lang.PkgValidateErrManifestFileOrKustomize, "nothing-there")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErrs == nil {
				require.NoError(t, err)
				return
			}
			for _, wantErr := range tt.wantErrs {
				require.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestValidateChart(t *testing.T) {
	longName := ""
	for range ZarfMaxChartNameLength + 1 {
		longName += "a"
	}
	tests := []struct {
		chart    ZarfChart
		wantErrs []string
		name     string
	}{
		{
			name:     "valid",
			chart:    ZarfChart{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
			wantErrs: nil,
		},
		{
			name:     "empty name",
			chart:    ZarfChart{Name: ""},
			wantErrs: []string{lang.PkgValidateErrChartNameMissing},
		},
		{
			name:  "long name",
			chart: ZarfChart{Name: longName},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartName, longName, ZarfMaxChartNameLength),
			},
		},
		{
			name:  "no url or local path",
			chart: ZarfChart{Name: "invalid"},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartNamespaceMissing, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
				fmt.Sprintf(lang.PkgValidateErrChartVersion, "invalid"),
			},
		},
		{
			name:  "both url and local path",
			chart: ZarfChart{Name: "invalid", Namespace: "whatever", URL: "http://whatever", LocalPath: "wherever"},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrChartURLOrPath, "invalid"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chart.Validate()
			if tt.wantErrs == nil {
				require.NoError(t, err)
				return
			}
			for _, wantErr := range tt.wantErrs {
				require.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestValidateComponentActions(t *testing.T) {
	tests := []struct {
		name     string
		actions  ZarfComponentActions
		wantErrs []string
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
			wantErrs: nil,
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
			wantErrs: []string{"cannot contain setVariables outside of onDeploy in actions"},
		},
		{
			name: "invalid onCreate action",
			actions: ZarfComponentActions{
				OnCreate: ZarfComponentActionSet{
					Before: []ZarfComponentAction{
						{
							Cmd:  "create",
							Wait: &ZarfComponentActionWait{},
						},
					},
				},
				OnDeploy: ZarfComponentActionSet{
					After: []ZarfComponentAction{
						{
							Cmd:  "deploy",
							Wait: &ZarfComponentActionWait{},
						},
					},
				},
				OnRemove: ZarfComponentActionSet{
					OnSuccess: []ZarfComponentAction{
						{
							Cmd:  "remove",
							Wait: &ZarfComponentActionWait{},
						},
					},
				},
			},
			wantErrs: []string{
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "create")).Error(),
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "deploy")).Error(),
				fmt.Errorf(lang.PkgValidateErrAction, fmt.Errorf(lang.PkgValidateErrActionCmdWait, "remove")).Error(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.actions.validate()
			if tt.wantErrs == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, wantErr := range tt.wantErrs {
				require.Contains(t, err.Error(), wantErr)
			}
		})
	}
}

func TestZarfComponentAction_Validate(t *testing.T) {
	tests := []struct {
		name        string
		action      ZarfComponentAction
		expectError bool
		wantErr     string
	}{
		{
			name:        "valid action no conditions",
			action:      ZarfComponentAction{},
			expectError: false,
		},
		{
			name: "cmd and wait both set",
			action: ZarfComponentAction{
				Cmd:  "ls",
				Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrActionCmdWait, "ls"),
		},
		{
			name: "cluster and network both set",
			action: ZarfComponentAction{
				Wait: &ZarfComponentActionWait{Cluster: &ZarfComponentActionWaitCluster{}, Network: &ZarfComponentActionWaitNetwork{}},
			},
			expectError: true,
			wantErr:     fmt.Sprintf(lang.PkgValidateErrActionClusterNetwork),
		},
		{
			name: "neither cluster nor network set",
			action: ZarfComponentAction{
				Wait: &ZarfComponentActionWait{},
			},
			expectError: true,
			wantErr:     lang.PkgValidateErrActionClusterNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action.Validate()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateZarfComponent(t *testing.T) {
	tests := []struct {
		component ZarfComponent
		wantErrs  []string
		name      string
	}{
		{
			name: "valid path",
			component: ZarfComponent{
				Name: "component1",
				Import: ZarfComponentImport{
					Path: "relative/path",
				},
			},
			wantErrs: nil,
		},
		{
			name: "valid URL",
			component: ZarfComponent{
				Name: "component2",
				Import: ZarfComponentImport{
					URL: "oci://example.com/package:v0.0.1",
				},
			},
			wantErrs: nil,
		},
		{
			name: "neither path nor URL provided",
			component: ZarfComponent{
				Name: "invalid1",
			},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "invalid1", "neither a path nor a URL was provided"),
			},
		},
		{
			name: "both path and URL provided",
			component: ZarfComponent{
				Name: "invalid2",
				Import: ZarfComponentImport{
					Path: "relative/path",
					URL:  "https://example.com",
				},
			},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "invalid2", "both a path and a URL were provided"),
			},
		},
		{
			name: "absolute path provided",
			component: ZarfComponent{
				Name: "invalid3",
				Import: ZarfComponentImport{
					Path: "/absolute/path",
				},
			},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "invalid3", "path cannot be an absolute path"),
			},
		},
		{
			name: "invalid URL provided",
			component: ZarfComponent{
				Name: "invalid4",
				Import: ZarfComponentImport{
					URL: "ftp://example.com",
				},
			},
			wantErrs: []string{
				fmt.Sprintf(lang.PkgValidateErrImportDefinition, "invalid4", "URL is not a valid OCI URL"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.component.Validate()
			if tt.wantErrs == nil {
				require.NoError(t, err)
				return
			}
			for _, wantErr := range tt.wantErrs {
				require.ErrorContains(t, err, wantErr)
			}
		})
	}
}
