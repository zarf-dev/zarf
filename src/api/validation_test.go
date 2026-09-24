// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestPackageValidateVersionFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pkg     api.Package
		wantErr string
	}{
		{
			name: "legacy package with omitted apiVersion",
			pkg:  api.Package{Components: []api.Component{{Images: []api.Image{{Name: "example.com/app:1"}}}}},
		},
		{
			name:    "legacy package rejects image source",
			pkg:     api.Package{Components: []api.Component{{Images: []api.Image{{Name: "example.com/app:1", Source: "daemon"}}}}},
			wantErr: "components[0].images[0].source is not supported in " + v1alpha1.APIVersion,
		},
		{
			name:    "v1alpha1 rejects service",
			pkg:     api.Package{APIVersion: v1alpha1.APIVersion, Components: []api.Component{{Service: "registry"}}},
			wantErr: "components[0].service",
		},
		{
			name: "v1alpha1 rejects multiple imports",
			pkg: api.Package{APIVersion: v1alpha1.APIVersion, Components: []api.Component{{
				Import: api.ComponentImport{Local: []api.ComponentImportLocal{{Path: "a"}, {Path: "b"}}},
			}}},
			wantErr: "components[0].import (multiple sources)",
		},
		{
			name: "v1beta1 accepts image source and legacy annotation aliases",
			pkg: api.Package{APIVersion: v1beta1.APIVersion, Metadata: api.PackageMetadata{
				URL: "https://example.com", Annotations: map[string]string{"other": "value"},
			}, Components: []api.Component{{Images: []api.Image{{Name: "example.com/app:1", Source: "daemon"}}}}},
		},
		{
			name: "v1beta1 rejects package legacy fields",
			pkg: api.Package{APIVersion: v1beta1.APIVersion,
				Variables: []api.InteractiveVariable{{}}, Constants: []api.Constant{{}}},
			wantErr: "variables is not supported in " + v1beta1.APIVersion,
		},
		{
			name: "v1beta1 rejects data injections",
			pkg: api.Package{APIVersion: v1beta1.APIVersion, Components: []api.Component{{
				DataInjections: []api.ZarfDataInjection{{Source: "file"}},
			}}},
			wantErr: "components[0].dataInjections",
		},
		{
			name: "v1beta1 rejects legacy chart fields",
			pkg: api.Package{APIVersion: v1beta1.APIVersion, Components: []api.Component{{
				Charts: []api.Chart{{LegacyVersion: "1.0.0"}},
			}}},
			wantErr: "components[0].charts[0].legacyVersion",
		},
		{
			name: "v1beta1 rejects legacy action fields",
			pkg: api.Package{APIVersion: v1beta1.APIVersion, Components: []api.Component{{
				Actions: api.ComponentActions{OnDeploy: api.ActionSet{
					After:  []api.Action{{Cmd: "echo after"}},
					Before: []api.Action{{SetVariables: []api.Variable{{Name: "OLD"}}}},
				}},
			}}},
			wantErr: "components[0].actions.onDeploy.after",
		},
		{
			name:    "unsupported version",
			pkg:     api.Package{APIVersion: "zarf.dev/future"},
			wantErr: "unsupported package apiVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.pkg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
