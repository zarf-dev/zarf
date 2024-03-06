// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"os"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/types"
)

func TestPackager_writeYaml(t *testing.T) {
	t.Parallel()
	type fields struct {
		cfg            *types.PackagerConfig
		cluster        *cluster.Cluster
		layout         *layout.PackagePaths
		arch           string
		warnings       []string
		valueTemplate  *template.Values
		hpaModified    bool
		connectStrings types.ConnectStrings
		sbomViewFiles  []string
		source         sources.PackageSource
		generation     int
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Test writeYaml with valid fields",
			fields: fields{
				cfg:            &types.PackagerConfig{},
				cluster:        &cluster.Cluster{},
				layout:         createTempLayout(),
				arch:           "amd64",
				warnings:       []string{},
				valueTemplate:  &template.Values{},
				hpaModified:    false,
				connectStrings: types.ConnectStrings{},
				sbomViewFiles:  []string{},
				generation:     1,
			},
			wantErr: false,
		},
		{
			name: "Test writeYaml with basic fields",
			fields: fields{
				cfg:            &types.PackagerConfig{},
				cluster:        nil,
				layout:         createTempLayout(),
				arch:           "",
				warnings:       nil,
				valueTemplate:  nil,
				hpaModified:    false,
				connectStrings: types.ConnectStrings{},
				sbomViewFiles:  nil,
				generation:     0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Packager{
				cfg:            tt.fields.cfg,
				cluster:        tt.fields.cluster,
				layout:         tt.fields.layout,
				arch:           tt.fields.arch,
				warnings:       tt.fields.warnings,
				valueTemplate:  tt.fields.valueTemplate,
				hpaModified:    tt.fields.hpaModified,
				connectStrings: tt.fields.connectStrings,
				sbomViewFiles:  tt.fields.sbomViewFiles,
				source:         tt.fields.source,
				generation:     tt.fields.generation,
			}
			err := p.writeYaml()
			if (err != nil) != tt.wantErr {
				t.Errorf("writeYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				_, err := os.Stat(p.layout.ZarfYAML)
				if os.IsNotExist(err) {
					t.Errorf("writeYaml() file not found, want file")
				} else if err != nil {
					t.Errorf("writeYaml() unexpected error = %v", err)
				}
			}
		})
	}
}

func createTempLayout() *layout.PackagePaths {
	tempDir, _ := os.MkdirTemp("", "zarf")
	tempFile, _ := os.CreateTemp(tempDir, "zarf*.yaml")
	return &layout.PackagePaths{
		ZarfYAML: tempFile.Name(),
	}
}

func TestPackager_filterComponents(t *testing.T) {
	t.Parallel()
	type fields struct {
		cfg            *types.PackagerConfig
		cluster        *cluster.Cluster
		layout         *layout.PackagePaths
		arch           string
		warnings       []string
		valueTemplate  *template.Values
		hpaModified    bool
		connectStrings types.ConnectStrings
		sbomViewFiles  []string
		source         sources.PackageSource
		generation     int
	}
	tests := []struct {
		name    string
		fields  fields
		count   int
		wantErr bool
	}{
		{
			name: "Test Case 1: Valid Packager",
			fields: fields{
				cfg: &types.PackagerConfig{
					Pkg: types.ZarfPackage{
						Components: []types.ZarfComponent{
							{
								Name: "test-component",
								Only: types.ZarfComponentOnlyTarget{
									Cluster: types.ZarfComponentOnlyCluster{
										Architecture: "",
									},
									LocalOS: "",
								},
							},
						},
					},
				},
				cluster:        &cluster.Cluster{},
				layout:         &layout.PackagePaths{},
				arch:           "amd64",
				warnings:       []string{},
				valueTemplate:  &template.Values{},
				hpaModified:    false,
				connectStrings: types.ConnectStrings{},
				sbomViewFiles:  []string{},
				generation:     1,
			},
			count:   1,
			wantErr: false,
		},

		{
			name: "Test Case 2: Invalid Packager",
			fields: fields{
				cfg: &types.PackagerConfig{
					Pkg: types.ZarfPackage{
						Components: []types.ZarfComponent{
							{
								Name: "invalid-component",
								Only: types.ZarfComponentOnlyTarget{
									Cluster: types.ZarfComponentOnlyCluster{
										Architecture: "invalid-arch",
									},
									LocalOS: "invalid-os",
								},
							},
						},
					},
				},
				cluster:        &cluster.Cluster{},
				layout:         &layout.PackagePaths{},
				arch:           "amd64",
				warnings:       []string{},
				valueTemplate:  &template.Values{},
				hpaModified:    false,
				connectStrings: types.ConnectStrings{},
				sbomViewFiles:  []string{},
				generation:     1,
			},
			count:   0,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Packager{
				cfg:            tt.fields.cfg,
				cluster:        tt.fields.cluster,
				layout:         tt.fields.layout,
				arch:           tt.fields.arch,
				warnings:       tt.fields.warnings,
				valueTemplate:  tt.fields.valueTemplate,
				hpaModified:    tt.fields.hpaModified,
				connectStrings: tt.fields.connectStrings,
				sbomViewFiles:  tt.fields.sbomViewFiles,
				source:         tt.fields.source,
				generation:     tt.fields.generation,
			}
			p.filterComponents()
			if len(p.cfg.Pkg.Components) != tt.count {
				t.Errorf("filterComponents() count = %v, want count %v", len(p.cfg.Pkg.Components), tt.count)
			}
		})
	}
}

func TestPackager_readZarfYAML(t *testing.T) {
	tempFile, err := os.CreateTemp("", "zarf-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	yamlContent := `
kind: ZarfPackageConfig
metadata:
  name: test-read-zarf-yaml
  description: Package to test reading Zarf YAML functionality
  architecture: amd64
build:
  migrations:
    - "example-migration-1"
    - "example-migration-2"
  architecture: amd64
  user: test-user
  terminal: test-terminal
  timestamp: "2023-04-01T00:00:00Z"
  version: "0.9.0"
components:
  - name: component-1
    description: An example component before migration
  - name: component-2
    description: Another example component before migration
`
	if _, err := tempFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Initialize the Packager struct properly
	p := &Packager{
		cfg:            &types.PackagerConfig{}, // Initialize cfg to avoid nil pointer dereference
		cluster:        &cluster.Cluster{},
		layout:         &layout.PackagePaths{},
		arch:           "amd64",
		warnings:       []string{},
		valueTemplate:  &template.Values{},
		hpaModified:    false,
		connectStrings: types.ConnectStrings{},
		sbomViewFiles:  []string{},
		generation:     1,
	}

	if err := p.readZarfYAML(tempFile.Name()); err != nil {
		t.Errorf("readZarfYAML() error = %v, wantErr false", err)
	}
}
