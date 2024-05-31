// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"fmt"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/stretchr/testify/assert"
)

func TestZarfPackageValidate(t *testing.T) {
	tests := []struct {
		name    string
		pkg     ZarfPackage
		wantErr string
	}{
		{
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
			wantErr: "",
		},
		{
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "empty-components",
				},
				Components: []ZarfComponent{},
			},
			wantErr: "package must have at least 1 component",
		},
		{
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
			//TODO refactor this to only have to contain each error so order doesn't matter
			wantErr: strings.Join([]string{
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
			}, "\n"),
		},
		{
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
			wantErr: strings.Join([]string{
				lang.PkgValidateErrInitNoYOLO,
				lang.PkgValidateErrYOLONoOCI,
				lang.PkgValidateErrYOLONoGit,
				lang.PkgValidateErrYOLONoArch,
				lang.PkgValidateErrYOLONoDistro,
			}, "\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.pkg.Metadata.Name, func(t *testing.T) {
			err := tt.pkg.Validate()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
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
		wantErr  string
	}{
		{
			manifest: ZarfManifest{Name: "valid", Files: []string{"a-file"}},
			wantErr:  "",
		},
		{
			manifest: ZarfManifest{Name: "", Files: []string{"a-file"}},
			wantErr:  lang.PkgValidateErrManifestNameMissing,
		},
		{
			manifest: ZarfManifest{Name: longName, Files: []string{"a-file"}},
			wantErr:  fmt.Sprintf(lang.PkgValidateErrManifestNameLength, longName, ZarfMaxChartNameLength),
		},
		{
			manifest: ZarfManifest{Name: "nothing-there"},
			wantErr:  fmt.Sprintf(lang.PkgValidateErrManifestFileOrKustomize, "nothing-there"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.manifest.Name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
