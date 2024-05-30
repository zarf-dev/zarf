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
				Kind: ZarfInitConfig,
				Metadata: ZarfMetadata{
					Name: "no-init-yolo",
					YOLO: true,
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
					},
				},
			},
			wantErr: lang.PkgValidateErrInitNoYOLO,
		},
		{
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "-invalid-package",
				},
				Components: []ZarfComponent{
					{Name: "component1"},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrPkgName, "-invalid-package"),
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
					Name: "bad-var",
				},
				Components: []ZarfComponent{
					{Name: "component1"},
				},
				Variables: []variables.InteractiveVariable{
					{
						Variable: variables.Variable{Name: "not_uppercase"},
					},
				},
			},
			wantErr: fmt.Errorf(lang.PkgValidateErrVariable, fmt.Errorf(lang.PkgValidateMustBeUppercase, "not_uppercase")).Error(),
		},
		{
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "bad-constant",
				},
				Components: []ZarfComponent{
					{Name: "component1"},
				},
				Constants: []variables.Constant{
					{
						Name: "not_uppercase",
					},
				},
			},
			wantErr: fmt.Errorf(lang.PkgValidateErrConstant, fmt.Errorf(lang.PkgValidateErrPkgConstantName, "not_uppercase")).Error(),
		},
		{
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "bad-constant-pattern",
				},
				Components: []ZarfComponent{
					{Name: "component1"},
				},
				Constants: []variables.Constant{
					{
						Name:    "BAD",
						Pattern: "^good_val$",
						Value:   "bad_val",
					},
				},
			},
			wantErr: fmt.Errorf(lang.PkgValidateErrConstant, fmt.Errorf(lang.PkgValidateErrPkgConstantPattern, "BAD", "^good_val$")).Error(),
		},
		{
			name: "duplicate component names",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
					},
					{
						Name: "component1",
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrComponentNameNotUnique, "component1"),
		},
		{
			name: "invalid component name",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "Component1",
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrComponentName, "Component1"),
		},
		{
			name: "unsupported OS",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
						Only: ZarfComponentOnlyTarget{
							LocalOS: "unsupportedOS",
						},
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrComponentLocalOS, "component1", "unsupportedOS", supportedOS),
		},
		{
			name: "required component with default",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name:     "component1",
						Default:  true,
						Required: helpers.BoolPtr(true),
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrComponentReqDefault, "component1"),
		},
		{
			name: "required component in group",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name:            "component1",
						Required:        helpers.BoolPtr(true),
						DeprecatedGroup: "group1",
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrComponentReqGrouped, "component1"),
		},
		{
			name: "duplicate chart names",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
						Charts: []ZarfChart{
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
							{Name: "chart1", Namespace: "whatever", URL: "http://whatever", Version: "v1.0.0"},
						},
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrChartNameNotUnique, "chart1"),
		},
		{
			name: "duplicate manifest names",
			pkg: ZarfPackage{
				Kind: ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Name: "valid-package",
				},
				Components: []ZarfComponent{
					{
						Name: "component1",
						Manifests: []ZarfManifest{
							{Name: "manifest1", Files: []string{"file1"}},
							{Name: "manifest1", Files: []string{"file2"}},
						},
					},
				},
			},
			wantErr: fmt.Sprintf(lang.PkgValidateErrManifestNameNotUnique, "manifest1"),
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
