// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
)

func TestParsePruneScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		raw         string
		cmd         PruneCommand
		expected    PruneScopes
		expectedErr string
	}{
		{
			name:     "empty string deploy",
			raw:      "",
			cmd:      PruneCommandDeploy,
			expected: nil,
		},
		{
			name:     "whitespace string deploy",
			raw:      "   ",
			cmd:      PruneCommandDeploy,
			expected: nil,
		},
		{
			name:     "empty string remove",
			raw:      "",
			cmd:      PruneCommandRemove,
			expected: nil,
		},
		{
			name:     "all deploy",
			raw:      "all",
			cmd:      PruneCommandDeploy,
			expected: PruneScopes{PruneScopeClusterResources, PruneScopeImages},
		},
		{
			name:     "all remove",
			raw:      "all",
			cmd:      PruneCommandRemove,
			expected: PruneScopes{PruneScopeImages},
		},
		{
			name:     "explicit cluster-resources deploy",
			raw:      "cluster-resources",
			cmd:      PruneCommandDeploy,
			expected: PruneScopes{PruneScopeClusterResources},
		},
		{
			name:     "explicit images deploy",
			raw:      "images",
			cmd:      PruneCommandDeploy,
			expected: PruneScopes{PruneScopeImages},
		},
		{
			name:     "multiple comma-separated deploy",
			raw:      "cluster-resources,images",
			cmd:      PruneCommandDeploy,
			expected: PruneScopes{PruneScopeClusterResources, PruneScopeImages},
		},
		{
			name:     "multiple with whitespace and deduplication",
			raw:      " images , cluster-resources , images ",
			cmd:      PruneCommandDeploy,
			expected: PruneScopes{PruneScopeImages, PruneScopeClusterResources},
		},
		{
			name:     "explicit images remove",
			raw:      "images",
			cmd:      PruneCommandRemove,
			expected: PruneScopes{PruneScopeImages},
		},
		{
			name:        "cluster-resources on remove is not applicable",
			raw:         "cluster-resources",
			cmd:         PruneCommandRemove,
			expectedErr: `prune scope "cluster-resources" is not applicable to package remove`,
		},
		{
			name:        "unknown scope deploy",
			raw:         "invalid-scope",
			cmd:         PruneCommandDeploy,
			expectedErr: `unknown prune scope "invalid-scope"`,
		},
		{
			name:        "unknown scope remove",
			raw:         "invalid-scope",
			cmd:         PruneCommandRemove,
			expectedErr: `unknown prune scope "invalid-scope"`,
		},
		{
			name:        "unknown command",
			raw:         "all",
			cmd:         "invalid-cmd",
			expectedErr: `unknown prune command "invalid-cmd"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual, err := ParsePruneScopes(tt.raw, tt.cmd)
			if tt.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestPruneScopesHelpers(t *testing.T) {
	t.Parallel()

	empty := PruneScopes{}
	require.True(t, empty.Empty())
	require.False(t, empty.ClusterResources())
	require.False(t, empty.Images())
	require.NoError(t, empty.Validate(PruneCommandDeploy))
	require.NoError(t, empty.Validate(PruneCommandRemove))

	deployAll := PruneScopes{PruneScopeClusterResources, PruneScopeImages}
	require.False(t, deployAll.Empty())
	require.True(t, deployAll.ClusterResources())
	require.True(t, deployAll.Images())
	require.True(t, deployAll.Contains(PruneScopeClusterResources))
	require.True(t, deployAll.Contains(PruneScopeImages))
	require.False(t, deployAll.Contains(PruneScope("other")))
	require.NoError(t, deployAll.Validate(PruneCommandDeploy))
	require.Error(t, deployAll.Validate(PruneCommandRemove))

	removeImages := PruneScopes{PruneScopeImages}
	require.False(t, removeImages.Empty())
	require.False(t, removeImages.ClusterResources())
	require.True(t, removeImages.Images())
	require.NoError(t, removeImages.Validate(PruneCommandRemove))

	invalidScope := PruneScopes{PruneScope("bogus")}
	require.Error(t, invalidScope.Validate(PruneCommandDeploy))
}

func TestDeployOptionsValidatePruneScopes(t *testing.T) {
	t.Parallel()
	pkg := api.Package{}
	opts := DeployOptions{
		PruneScopes: PruneScopes{PruneScopeClusterResources, PruneScopeImages},
	}
	require.NoError(t, opts.Validate(pkg))

	invalidOpts := DeployOptions{
		PruneScopes: PruneScopes{PruneScope("invalid")},
	}
	require.Error(t, invalidOpts.Validate(pkg))
}
