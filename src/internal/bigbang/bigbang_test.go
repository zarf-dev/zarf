// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package bigbang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRequiredBigBangVersions(t *testing.T) {
	// Support 1.54.0 and beyond
	vv, err := isValidVersion("1.54.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Do not support earlier than 1.54.0
	vv, err = isValidVersion("1.53.0")
	require.NoError(t, err)
	require.False(t, vv)

	// Support for Big Bang release candidates
	vv, err = isValidVersion("1.57.0-rc.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Support for Big Bang 2.0.0
	vv, err = isValidVersion("2.0.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Fail on non-semantic versions
	vv, err = isValidVersion("1.57b")
	require.EqualError(t, err, "Invalid Semantic Version")
	require.False(t, vv)
}

func TestFindBBResources(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "find-bb-template-resources.yaml"))
	require.NoError(t, err)
	template := string(b)
	tests := []struct {
		name                      string
		input                     string
		expectedGitRepos          map[string]string
		expectedHelmReleaseDeps   map[string]HelmReleaseDependency
		expectedHelmReleaseValues map[string]map[string]interface{}
		expectedErr               bool
	}{
		{
			name:  "Valid input with HelmRelease, GitRepository, Secret, and ConfigMap",
			input: template,
			expectedGitRepos: map[string]string{
				"default.my-git-repo": "https://github.com/example/repo.git@main",
			},
			expectedHelmReleaseDeps: map[string]HelmReleaseDependency{
				"default.my-helm-release": {
					Metadata: metav1.ObjectMeta{
						Name:      "my-helm-release",
						Namespace: "default",
					},
					NamespacedDependencies: []string{"istio.another-helm-release"},
					NamespacedSource:       "default.my-git-repo",
					ValuesFrom: []v2beta1.ValuesReference{
						{
							Kind: "ConfigMap",
							Name: "my-configmap",
						},
						{
							Kind: "Secret",
							Name: "my-secret",
						},
					},
				},
			},
			expectedHelmReleaseValues: map[string]map[string]interface{}{
				"default.my-helm-release": {
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedErr: false,
		},
		{
			name: "Invalid YAML input",
			input: `
		invalid-yaml
		`,
			expectedGitRepos:          nil,
			expectedHelmReleaseDeps:   nil,
			expectedHelmReleaseValues: nil,
			expectedErr:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitRepos, helmReleaseDeps, helmReleaseValues, err := findBBResources(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedGitRepos, gitRepos)
				require.Equal(t, tt.expectedHelmReleaseDeps, helmReleaseDeps)
				require.Equal(t, tt.expectedHelmReleaseValues, helmReleaseValues)
			}
		})
	}
}
