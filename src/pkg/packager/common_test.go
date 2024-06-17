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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/types"
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

			cs := fake.NewSimpleClientset()

			p := &Packager{
				cluster: &cluster.Cluster{
					Clientset: cs,
				},
				cfg: &types.PackagerConfig{
					Pkg: types.ZarfPackage{
						Metadata: types.ZarfMetadata{Architecture: tt.pkgArch},
						Components: []types.ZarfComponent{
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

	type testCase struct {
		name                   string
		cliVersion             string
		lastNonBreakingVersion string
		expectedErrorMessage   string
		expectedWarningMessage string
		returnError            bool
		throwWarning           bool
	}

	testCases := []testCase{
		{
			name:                   "CLI version less than lastNonBreakingVersion",
			cliVersion:             "v0.26.4",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           true,
			expectedWarningMessage: fmt.Sprintf(
				lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
				"v0.26.4",
				"v0.27.0",
				"v0.27.0",
			),
		},
		{
			name:                   "invalid semantic version (CLI version)",
			cliVersion:             "invalidSemanticVersion",
			lastNonBreakingVersion: "v0.0.1",
			returnError:            false,
			throwWarning:           true,
			expectedWarningMessage: fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, "invalidSemanticVersion"),
		},
		{
			name:                   "invalid semantic version (lastNonBreakingVersion)",
			cliVersion:             "v0.0.1",
			lastNonBreakingVersion: "invalidSemanticVersion",
			throwWarning:           false,
			returnError:            true,
			expectedErrorMessage:   "unable to parse lastNonBreakingVersion",
		},
		{
			name:                   "CLI version greater than lastNonBreakingVersion",
			cliVersion:             "v0.28.2",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           false,
		},
		{
			name:                   "CLI version equal to lastNonBreakingVersion",
			cliVersion:             "v0.27.0",
			lastNonBreakingVersion: "v0.27.0",
			returnError:            false,
			throwWarning:           false,
		},
		{
			name:                   "empty lastNonBreakingVersion",
			cliVersion:             "this shouldn't get evaluated when the lastNonBreakingVersion is empty",
			lastNonBreakingVersion: "",
			returnError:            false,
			throwWarning:           false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			config.CLIVersion = testCase.cliVersion

			p := &Packager{
				cfg: &types.PackagerConfig{
					Pkg: types.ZarfPackage{
						Build: types.ZarfBuildData{
							LastNonBreakingVersion: testCase.lastNonBreakingVersion,
						},
					},
				},
			}

			err := p.validateLastNonBreakingVersion()

			switch {
			case testCase.returnError:
				require.ErrorContains(t, err, testCase.expectedErrorMessage)
				require.Empty(t, p.warnings, "Expected no warnings for test case: %s", testCase.name)
			case testCase.throwWarning:
				require.Contains(t, p.warnings, testCase.expectedWarningMessage)
				require.NoError(t, err, "Expected no error for test case: %s", testCase.name)
			default:
				require.NoError(t, err, "Expected no error for test case: %s", testCase.name)
				require.Empty(t, p.warnings, "Expected no warnings for test case: %s", testCase.name)
			}
		})
	}
}
