// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package kustomize

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		path                       string
		kustomizeAllowAnyDirectory bool
		enableKustomizePlugins     bool
	}{
		{
			path:                       path.Join("testdata", "generators"),
			kustomizeAllowAnyDirectory: false,
			enableKustomizePlugins:     false,
		},
		{
			path:                       path.Join("testdata", "helm"),
			kustomizeAllowAnyDirectory: true,
			enableKustomizePlugins:     true,
		},
	}

	for _, test := range tests {
		tmpdir := t.TempDir()

		builtManifest := path.Join(tmpdir, "generated.yaml")

		err := Build(test.path, builtManifest, test.kustomizeAllowAnyDirectory, test.enableKustomizePlugins)
		require.NoError(t, err)
		require.FileExists(t, builtManifest, "built manifest file should exist")

		expectedContent, err := os.ReadFile(path.Join(test.path, "expected.yaml"))
		buildContent, err := os.ReadFile(builtManifest)
		require.NoError(t, err)

		require.Equalf(t, expectedContent, buildContent, "Built kustomization should match expected")
	}
}
