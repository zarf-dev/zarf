// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestValidatePackage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		pkg          v1beta1.Package
		expectedErrs []string
	}{
		{
			name: "valid package",
			pkg: v1beta1.Package{
				Kind: v1beta1.ZarfPackageConfig,
				Metadata: v1beta1.PackageMetadata{
					Name: "valid-package",
				},
				Components: []v1beta1.Component{
					{
						Name: "component1",
					},
				},
			},
			expectedErrs: nil,
		},
		{
			name: "no components",
			pkg: v1beta1.Package{
				Kind: v1beta1.ZarfPackageConfig,
				Metadata: v1beta1.PackageMetadata{
					Name: "valid-package",
				},
			},
			expectedErrs: []string{PkgValidateErrNoComponents},
		},
		{
			name: "invalid package",
			pkg: v1beta1.Package{
				Kind: v1beta1.ZarfPackageConfig,
				Metadata: v1beta1.PackageMetadata{
					Name: "invalid-package",
				},
				Components: []v1beta1.Component{
					{
						Name: "invalid",
						ComponentSpec: v1beta1.ComponentSpec{
							Charts: []v1beta1.Chart{
								{Name: "chart1", Namespace: "whatever", Local: &v1beta1.LocalSource{Path: "whatever"}},
								{Name: "chart1", Namespace: "whatever", Local: &v1beta1.LocalSource{Path: "whatever"}},
							},
							Manifests: []v1beta1.Manifest{
								{Name: "manifest1", Files: []string{"file1"}},
								{Name: "manifest1", Files: []string{"file2"}},
							},
						},
					},
					{
						Name: "duplicate",
					},
					{
						Name: "duplicate",
					},
				},
			},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartNameNotUnique, "chart1"),
				fmt.Sprintf(PkgValidateErrManifestNameNotUnique, "manifest1"),
				fmt.Sprintf(PkgValidateErrComponentNameNotUnique, "duplicate"),
			},
		},
	}

	for _, tt := range tests {
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
		manifest     v1beta1.Manifest
		expectedErrs []string
		name         string
	}{
		{
			name:         "valid files",
			manifest:     v1beta1.Manifest{Name: "valid", Files: []string{"a-file"}},
			expectedErrs: nil,
		},
		{
			name:         "valid kustomize",
			manifest:     v1beta1.Manifest{Name: "valid", Kustomize: &v1beta1.KustomizeManifest{Files: []string{"a-dir"}}},
			expectedErrs: nil,
		},
		{
			name:         "long name",
			manifest:     v1beta1.Manifest{Name: longName, Files: []string{"a-file"}},
			expectedErrs: []string{fmt.Sprintf(PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength)},
		},
		{
			name:         "no files or kustomize",
			manifest:     v1beta1.Manifest{Name: "nothing-there"},
			expectedErrs: []string{fmt.Sprintf(PkgValidateErrManifestFileOrKustomize, "nothing-there")},
		},
	}
	for _, tt := range tests {
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
	}

	for _, tt := range tests {
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
		chart        v1beta1.Chart
		expectedErrs []string
		partialMatch bool
	}{
		{
			name:         "valid",
			chart:        v1beta1.Chart{Name: "chart1", Namespace: "whatever", Local: &v1beta1.LocalSource{Path: "whatever"}, ReleaseName: "this-is-valid"},
			expectedErrs: nil,
		},
		{
			name:  "long name",
			chart: v1beta1.Chart{Name: longName, Namespace: "whatever", Local: &v1beta1.LocalSource{Path: "whatever"}},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartName, longName, ZarfMaxChartNameLength),
			},
		},
		{
			name:  "no namespace or source",
			chart: v1beta1.Chart{Name: "invalid"},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartNamespaceMissing, "invalid"),
				fmt.Sprintf(PkgValidateErrChartSource, "invalid"),
			},
		},
		{
			name: "multiple sources",
			chart: v1beta1.Chart{
				Name: "invalid", Namespace: "whatever",
				Local: &v1beta1.LocalSource{Path: "whatever"},
				OCI:   &v1beta1.OCISource{URL: "oci://whatever", Ref: v1beta1.OCIRef{Tag: "1.0.0"}},
			},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrChartSource, "invalid"),
			},
		},
		{
			name:         "invalid releaseName",
			chart:        v1beta1.Chart{ReleaseName: "namedwithperiods-0.47.0", Name: "releaseName", Namespace: "whatever", Local: &v1beta1.LocalSource{Path: "whatever"}},
			expectedErrs: []string{"invalid release name 'namedwithperiods-0.47.0'"},
			partialMatch: true,
		},
		{
			name:         "missing releaseName fallsback to name",
			chart:        v1beta1.Chart{Name: "chart3", Namespace: "namespace", Local: &v1beta1.LocalSource{Path: "whatever"}},
			expectedErrs: nil,
		},
		{
			name:         "missing name and releaseName",
			chart:        v1beta1.Chart{Namespace: "namespace", Local: &v1beta1.LocalSource{Path: "whatever"}},
			expectedErrs: []string{errChartReleaseNameEmpty},
		},
	}
	for _, tt := range tests {
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
		actions      v1beta1.ComponentActions
		expectedErrs []string
	}{
		{
			name: "valid actions",
			actions: v1beta1.ComponentActions{
				OnCreate: v1beta1.ComponentActionSet{
					Before: []v1beta1.ComponentAction{
						{
							Cmd: "echo 'onCreate before valid'",
						},
					},
				},
				OnDeploy: v1beta1.ComponentActionSet{
					Before: []v1beta1.ComponentAction{
						{
							Cmd: "echo 'onDeploy before valid'",
						},
					},
				},
			},
			expectedErrs: nil,
		},
		{
			name: "setValues in onCreate",
			actions: v1beta1.ComponentActions{
				OnCreate: v1beta1.ComponentActionSet{
					Before: []v1beta1.ComponentAction{
						{
							Cmd:       "echo 'invalid setValue'",
							SetValues: []v1beta1.SetValue{{Key: "key"}},
						},
					},
				},
			},
			expectedErrs: []string{PkgValidateErrActionSetValueOnDeploy},
		},
		{
			name: "templating in onCreate",
			actions: v1beta1.ComponentActions{
				OnCreate: v1beta1.ComponentActionSet{
					Before: []v1beta1.ComponentAction{
						{
							Cmd:              "echo 'templating not allowed'",
							EnableTemplating: true,
						},
					},
				},
			},
			expectedErrs: []string{PkgValidateErrActionTemplateOnCreate},
		},
		{
			name: "invalid actions",
			actions: v1beta1.ComponentActions{
				OnCreate: v1beta1.ComponentActionSet{
					Before: []v1beta1.ComponentAction{
						{
							Cmd:  "create",
							Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{}},
						},
					},
				},
				OnRemove: v1beta1.ComponentActionSet{
					OnSuccess: []v1beta1.ComponentAction{
						{
							Cmd:  "remove",
							Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{}},
						},
					},
					OnFailure: []v1beta1.ComponentAction{
						{
							Cmd:  "remove2",
							Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{}},
						},
					},
				},
			},
			expectedErrs: []string{
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "create")).Error(),
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "remove")).Error(),
				fmt.Errorf(PkgValidateErrAction, fmt.Errorf(PkgValidateErrActionCmdWait, "remove2")).Error(),
			},
		},
	}

	for _, tt := range tests {
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
		action       v1beta1.ComponentAction
		expectedErrs []string
	}{
		{
			name:   "valid action no conditions",
			action: v1beta1.ComponentAction{},
		},
		{
			name: "cmd and wait both set, nothing in wait",
			action: v1beta1.ComponentAction{
				Cmd:  "ls",
				Wait: &v1beta1.ComponentActionWait{},
			},
			expectedErrs: []string{
				fmt.Sprintf(PkgValidateErrActionCmdWait, "ls"),
				PkgValidateErrActionClusterNetwork,
			},
		},
		{
			name: "cluster and network both set",
			action: v1beta1.ComponentAction{
				Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{}, Network: &v1beta1.ComponentActionWaitNetwork{}},
			},
			expectedErrs: []string{PkgValidateErrActionClusterNetwork},
		},
	}

	for _, tt := range tests {
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
