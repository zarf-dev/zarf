// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package kustomize

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func TestRunKustomizer_recoversPanic(t *testing.T) {
	t.Parallel()
	fs := filesys.MakeFsInMemory()
	require.NoError(t, fs.WriteFile("/k/kustomization.yaml", []byte(`resources: [resources.yaml]
patches:
  - target: {kind: ConfigMap}
    patch: |
      - op: replace
        path: /metadata/name
        value: same
`)))
	require.NoError(t, fs.WriteFile("/k/resources.yaml", []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: a
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: b
`)))
	// Renaming both ConfigMaps to the same name makes krusty panic.
	_, err := RunKustomizer(fs, "/k", krusty.MakeDefaultOptions())
	require.ErrorContains(t, err, "recovered from kustomize build panic")
	require.ErrorContains(t, err, "already registered id")

	// The lock is released after a panic.
	require.NoError(t, fs.WriteFile("/k/kustomization.yaml", []byte("resources: [resources.yaml]\n")))
	resMap, err := RunKustomizer(fs, "/k", krusty.MakeDefaultOptions())
	require.NoError(t, err)
	require.Equal(t, 2, resMap.Size())
}
