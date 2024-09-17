// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package bigbang

import (
	"testing"

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
	tests := []struct {
		name                      string
		input                     string
		expectedGitRepos          map[string]string
		expectedHelmReleaseDeps   map[string]HelmReleaseDependency
		expectedHelmReleaseValues map[string]map[string]interface{}
		expectedErr               bool
	}{
		{
			name: "Valid input with HelmRelease, GitRepository, Secret, and ConfigMap",
			input: `
apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-helm-release
  namespace: default
spec:
  chart:
    spec:
      sourceRef:
        kind: GitRepository
        name: my-git-repo
        namespace: default
  dependsOn:
  - name: another-helm-release
    namespace: istio
---
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: GitRepository
metadata:
  name: my-git-repo
  namespace: default
spec:
  url: https://github.com/example/repo.git
  ref:
    branch: main
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
type: Opaque
data:
  key: dmFsdWU=
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: default
data:
  key: value
`,
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
					ValuesFrom:             nil,
				},
			},
			expectedHelmReleaseValues: map[string]map[string]interface{}{
				"default.my-helm-release": {},
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
