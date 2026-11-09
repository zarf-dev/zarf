// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestDevUpgradeTemporaryFileDestinations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zarf.yaml"), []byte(`kind: ZarfPackageConfig
metadata:
  name: temporary-files
components:
  - name: first
    files:
      - source: payload
        target: "###ZARF_TEMP###/payload"
      - source: plain
        target: /opt/plain
  - name: second
    files:
      - source: binary
        target: "prefix/###ZARF_TEMP###/binary"
        template: false
`), 0o600))

	var output bytes.Buffer
	cmd := newDevUpgradeSchemaCommand()
	cmd.SetArgs([]string{dir})
	cmd.SetOut(&output)
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	var pkg v1beta1.Package
	require.NoError(t, yaml.Unmarshal(output.Bytes(), &pkg))
	require.Equal(t, v1beta1.APIVersion, pkg.APIVersion)
	require.Len(t, pkg.Components, 2)
	require.Len(t, pkg.Components[0].Files, 2)
	require.Equal(t, "{{ .Runtime.TempDir }}/payload", pkg.Components[0].Files[0].Destination)
	require.Equal(t, "/opt/plain", pkg.Components[0].Files[1].Destination)
	require.Len(t, pkg.Components[1].Files, 1)
	require.Equal(t, "prefix/{{ .Runtime.TempDir }}/binary", pkg.Components[1].Files[0].Destination)
	require.False(t, pkg.Components[1].Files[0].EnableTemplating)
}

func TestCheckRemovedFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		pkg     v1alpha1.ZarfPackage
		wantErr string
	}{
		{
			name:    "yolo",
			pkg:     v1alpha1.ZarfPackage{Metadata: v1alpha1.ZarfMetadata{YOLO: true}},
			wantErr: ".metadata.yolo",
		},
		{
			name:    "package variables",
			pkg:     v1alpha1.ZarfPackage{Variables: []v1alpha1.InteractiveVariable{{Variable: v1alpha1.Variable{Name: "FOO"}}}},
			wantErr: ".variables is removed",
		},
		{
			name:    "package constants",
			pkg:     v1alpha1.ZarfPackage{Constants: []v1alpha1.Constant{{Name: "FOO"}}},
			wantErr: ".constants is removed",
		},
		{
			name:    "component group",
			pkg:     v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "c", DeprecatedGroup: "g"}}},
			wantErr: ".components.group",
		},
		{
			name:    "component default",
			pkg:     v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "c", Default: true}}},
			wantErr: ".components.default",
		},
		{
			name:    "data injections",
			pkg:     v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "c", DataInjections: []v1alpha1.ZarfDataInjection{{Source: "/data"}}}}},
			wantErr: ".components.dataInjections",
		},
		{
			name: "cluster distro",
			pkg: v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{
				Name: "c",
				Only: v1alpha1.ZarfComponentOnlyTarget{Cluster: v1alpha1.ZarfComponentOnlyCluster{Distros: []string{"k3s"}}},
			}}},
			wantErr: ".components.only.cluster.distro",
		},
		{
			name:    "import name",
			pkg:     v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "c", Import: v1alpha1.ZarfComponentImport{Name: "n"}}}},
			wantErr: ".components.import.name",
		},
		{
			name:    "chart variables",
			pkg:     v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Name: "c", Charts: []v1alpha1.ZarfChart{{Name: "ch", Variables: []v1alpha1.ZarfChartVariable{{Name: "V"}}}}}}},
			wantErr: ".components.charts.variables",
		},
		{
			name: "action setVariables",
			pkg: v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{
				Name: "c",
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Before: []v1alpha1.ZarfComponentAction{{Cmd: "echo", SetVariables: []v1alpha1.Variable{{Name: "V"}}}},
					},
				},
			}}},
			wantErr: ".components.actions.onDeploy",
		},
		{
			name: "action deprecated setVariable",
			pkg: v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{
				Name: "c",
				Actions: v1alpha1.ZarfComponentActions{
					OnCreate: v1alpha1.ZarfComponentActionSet{
						OnSuccess: []v1alpha1.ZarfComponentAction{{Cmd: "echo", DeprecatedSetVariable: "V"}},
					},
				},
			}}},
			wantErr: ".components.actions.onCreate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkRemovedFields(tt.pkg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCheckRemovedFieldsConvertible(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "convertible",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:   "web",
				Images: []string{"nginx:latest"},
				Charts: []v1alpha1.ZarfChart{{Name: "ch", LocalPath: "./chart"}},
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Before: []v1alpha1.ZarfComponentAction{{Cmd: "echo hi", SetValues: []v1alpha1.SetValue{{Key: "k"}}}},
					},
				},
			},
		},
	}
	require.NoError(t, checkRemovedFields(pkg))
}

func TestValidateVersionUpgrade(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr string
	}{
		{
			name: "same version is valid",
			from: v1alpha1.APIVersion,
			to:   v1alpha1.APIVersion,
		},
		{
			name: "upgrade v1alpha1 to v1beta1",
			from: v1alpha1.APIVersion,
			to:   v1beta1.APIVersion,
		},
		{
			name:    "downgrade v1beta1 to v1alpha1",
			from:    v1beta1.APIVersion,
			to:      v1alpha1.APIVersion,
			wantErr: "cannot downgrade",
		},
		{
			name:    "unknown source version",
			from:    "zarf.dev/v99",
			to:      v1beta1.APIVersion,
			wantErr: "unsupported source API version",
		},
		{
			name:    "unknown target version",
			from:    v1alpha1.APIVersion,
			to:      "zarf.dev/v99",
			wantErr: "unsupported target API version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateVersionUpgrade(tt.from, tt.to)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
