// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package kustomize

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

func TestValidatePatch(t *testing.T) {
	t.Parallel()
	deployments := &types.Selector{ResId: resid.ResId{Gvk: resid.Gvk{Group: "apps", Version: "v1", Kind: "Deployment"}}}
	tests := []struct {
		name    string
		patch   string
		target  *types.Selector
		wantErr string
	}{
		{
			name:  "untargeted strategic merge patch",
			patch: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\nspec:\n  replicas: 2\n",
		},
		{
			name:   "targeted strategic merge patch",
			patch:  "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: ignored\nspec:\n  replicas: 2\n",
			target: deployments,
		},
		{
			name:  "untargeted multi-document strategic merge patch",
			patch: "kind: Deployment\nmetadata:\n  name: a\n---\nkind: Service\nmetadata:\n  name: b\n",
		},
		{
			name:  "strategic merge patch without apiVersion",
			patch: "kind: Deployment\nmetadata:\n  name: app\n",
		},
		{
			name:   "JSON 6902 patch with target",
			patch:  "- op: replace\n  path: /spec/replicas\n  value: 2\n",
			target: deployments,
		},
		{
			name:   "JSON 6902 patch as JSON with target",
			patch:  `[{"op": "replace", "path": "/spec/replicas", "value": 2}]`,
			target: deployments,
		},
		{
			name:  "List kind without name",
			patch: "apiVersion: v1\nkind: List\nitems: []\n",
		},
		{
			name:    "JSON 6902 patch without target",
			patch:   "- op: replace\n  path: /spec/replicas\n  value: 2\n",
			wantErr: "must specify a target for JSON patch",
		},
		{
			name:    "multi-document strategic merge patch with target",
			patch:   "kind: Deployment\nmetadata:\n  name: a\n---\nkind: Service\nmetadata:\n  name: b\n",
			target:  deployments,
			wantErr: "Multiple Strategic-Merge Patches in one `patches` entry is not allowed to set `patches.target`",
		},
		{
			name:    "strategic merge patch without kind",
			patch:   "metadata:\n  name: app\n",
			wantErr: "unable to parse SM or JSON patch",
		},
		{
			name:    "untargeted strategic merge patch without metadata.name",
			patch:   "kind: Deployment\nspec:\n  replicas: 2\n",
			wantErr: "unable to parse SM or JSON patch",
		},
		{
			name:    "targeted strategic merge patch without metadata.name",
			patch:   "kind: Deployment\nspec:\n  replicas: 2\n",
			target:  deployments,
			wantErr: "unable to parse SM or JSON patch",
		},
		{
			name:    "invalid JSON starting with a bracket",
			patch:   "[op: replace]",
			target:  deployments,
			wantErr: "unable to parse SM or JSON patch",
		},
		{
			name:    "scalar",
			patch:   "not a patch",
			target:  deployments,
			wantErr: "unable to parse SM or JSON patch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePatch(tt.patch, tt.target)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
			require.NotContains(t, err.Error(), "trouble configuring")
		})
	}
}
