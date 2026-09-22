// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestReadYaml_KubernetesTypeMeta(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join(t.TempDir(), "configmap.yaml")
	manifest := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: example\n")
	require.NoError(t, os.WriteFile(manifestPath, manifest, 0o600))

	var configMap corev1.ConfigMap
	require.NoError(t, ReadYaml(manifestPath, &configMap))
	require.Equal(t, "ConfigMap", configMap.Kind)
	require.Equal(t, "v1", configMap.APIVersion)
}
