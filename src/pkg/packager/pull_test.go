// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/types"
)

type MockPackageSource struct {
	ShouldFail bool
}

func (m *MockPackageSource) LoadPackage(dst *layout.PackagePaths, unarchiveAll bool) error {
	// Mock implementation
	return nil
}

func (m *MockPackageSource) LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) error {
	// Mock implementation
	return nil
}

func (m *MockPackageSource) Collect(destinationDirectory string) (tarball string, err error) {
	if m.ShouldFail {
		return "", fmt.Errorf("mock failure in Collect")
	}
	// Mock implementation
	return "mock-tarball.tar", nil
}

func TestPackager_Pull(t *testing.T) {
	mockSource := &MockPackageSource{}

	tests := []struct {
		name    string
		cfg     *types.PackagerConfig
		cluster *cluster.Cluster
		layout  *layout.PackagePaths
		arch    string
		source  sources.PackageSource
		wantErr bool
	}{
		{
			name:    "Successful pull with mock source",
			cfg:     &types.PackagerConfig{},
			cluster: &cluster.Cluster{},
			layout:  &layout.PackagePaths{},
			arch:    "amd64",
			source:  mockSource,
			wantErr: false,
		},
		{
			name:    "Failed pull with mock source",
			cfg:     &types.PackagerConfig{},
			cluster: &cluster.Cluster{},
			layout:  &layout.PackagePaths{},
			arch:    "amd64",
			source:  &MockPackageSource{ShouldFail: true},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Packager{
				cfg:            tt.cfg,
				cluster:        tt.cluster,
				layout:         tt.layout,
				arch:           tt.arch,
				warnings:       []string{},
				valueTemplate:  &template.Values{},
				hpaModified:    false,
				connectStrings: types.ConnectStrings{},
				sbomViewFiles:  []string{},
				source:         tt.source,
				generation:     1,
			}
			if err := p.Pull(); (err != nil) != tt.wantErr {
				t.Errorf("Pull() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
