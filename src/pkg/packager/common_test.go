// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/types"
)

func TestValidatePackageArchitecture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		pkgArch      string
		clusterArchs []string
		images       []string
		wantErr      error
	}{
		{
			name:         "architecture match",
			pkgArch:      "amd64",
			clusterArchs: []string{"amd64"},
			images:       []string{"nginx"},
			wantErr:      nil,
		},
		{
			name:         "architecture mismatch",
			pkgArch:      "arm64",
			clusterArchs: []string{"amd64"},
			images:       []string{"nginx"},
			wantErr:      fmt.Errorf(lang.CmdPackageDeployValidateArchitectureErr, "arm64", "amd64"),
		},
		{
			name:         "multiple cluster architectures",
			pkgArch:      "arm64",
			clusterArchs: []string{"amd64", "arm64"},
			images:       []string{"nginx"},
			wantErr:      nil,
		},
		{
			name:         "ignore validation when package arch equals 'multi'",
			pkgArch:      "multi",
			clusterArchs: []string{"not evaluated"},
			wantErr:      nil,
		},
		{
			name:         "ignore validation when a package doesn't contain images",
			pkgArch:      "amd64",
			images:       []string{},
			clusterArchs: []string{"not evaluated"},
			wantErr:      nil,
		},
		{
			name:    "test the error path when fetching cluster architecture fails",
			pkgArch: "amd64",
			images:  []string{"nginx"},
			wantErr: lang.ErrUnableToCheckArch,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cs := fake.NewClientset()

			p := &Packager{
				cluster: &cluster.Cluster{
					Clientset: cs,
				},
				cfg: &types.PackagerConfig{
					Pkg: v1alpha1.ZarfPackage{
						Metadata: v1alpha1.ZarfMetadata{Architecture: tt.pkgArch},
						Components: []v1alpha1.ZarfComponent{
							{
								Images: tt.images,
							},
						},
					},
				},
			}

			for i, arch := range tt.clusterArchs {
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("node-%d-%s", i, tt.name),
					},
					Status: v1.NodeStatus{
						NodeInfo: v1.NodeSystemInfo{
							Architecture: arch,
						},
					},
				}
				_, err := cs.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			err := p.validatePackageArchitecture(context.Background())
			require.Equal(t, tt.wantErr, err)
		})
	}
}

// TestValidateLastNonBreakingVersion verifies that Zarf validates the lastNonBreakingVersion of packages against the CLI version correctly.
func TestValidateLastNonBreakingVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		cliVersion             string
		lastNonBreakingVersion string
		expectedErr            string
		expectedWarnings       []string
	}{
		{
			name:                   "CLI version less than last non breaking version",
			cliVersion:             "v0.26.4",
			lastNonBreakingVersion: "v0.27.0",
			expectedWarnings: []string{
				fmt.Sprintf(
					lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
					"v0.26.4",
					"v0.27.0",
					"v0.27.0",
				),
			},
		},
		{
			name:                   "invalid cli version",
			cliVersion:             "invalidSemanticVersion",
			lastNonBreakingVersion: "v0.0.1",
			expectedWarnings:       []string{fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, "invalidSemanticVersion")},
		},
		{
			name:                   "invalid last non breaking version",
			cliVersion:             "v0.0.1",
			lastNonBreakingVersion: "invalidSemanticVersion",
			expectedErr:            "unable to parse last non breaking version",
		},
		{
			name:                   "CLI version greater than last non breaking version",
			cliVersion:             "v0.28.2",
			lastNonBreakingVersion: "v0.27.0",
		},
		{
			name:                   "CLI version equal to last non breaking version",
			cliVersion:             "v0.27.0",
			lastNonBreakingVersion: "v0.27.0",
		},
		{
			name:                   "empty last non breaking version",
			cliVersion:             "",
			lastNonBreakingVersion: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := validateLastNonBreakingVersion(tt.cliVersion, tt.lastNonBreakingVersion)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				require.Empty(t, warnings)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, tt.expectedWarnings, warnings)
		})
	}
}
