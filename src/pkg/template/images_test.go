// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

func TestApplyImageRepository(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		image    string
		expected string
	}{
		{image: "ghcr.io/team/agent:v1.2.3", expected: "team/agent"},
		{image: "registry.example:5443/team/agent:v1.2.3", expected: "team/agent"},
		{image: "[::1]:5000/team/agent:v1.2.3", expected: "team/agent"},
		{image: "localhost/agent:v1.2.3", expected: "agent"},
		{image: "library/registry:3.1.1", expected: "library/registry"},
		{image: "nginx", expected: "library/nginx"},
		{image: "nginx@sha256:" + strings.Repeat("a", 64), expected: "library/nginx"},
		{image: "nginx:1.27@sha256:" + strings.Repeat("a", 64), expected: "library/nginx"},
		{image: ""},
		{image: "nginx:"},
		{image: "https://ghcr.io/team/agent:v1.2.3"},
		{image: "nginx@sha256:invalid"},
	} {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()
			result, err := Apply(t.Context(), `{{ imageRepository .Values.image }}`, NewObjects(value.Values{"image": tt.image}))
			if tt.expected == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyImageTag(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		image    string
		expected string
	}{
		{image: "ghcr.io/team/agent:v1.2.3", expected: "v1.2.3"},
		{image: "registry.example:5443/team/agent:v1.2.3", expected: "v1.2.3"},
		{image: "registry.example:5443/team/agent", expected: "latest"},
		{image: "nginx", expected: "latest"},
		{image: ""},
		{image: "nginx:"},
		{image: "https://ghcr.io/team/agent:v1.2.3"},
		{image: "nginx@sha256:" + strings.Repeat("a", 64)},
		{image: "nginx:1.27@sha256:" + strings.Repeat("a", 64)},
	} {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()
			result, err := Apply(t.Context(), `{{ imageTag .Values.image }}`, NewObjects(value.Values{"image": tt.image}))
			if tt.expected == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyImageTagOrDigest(t *testing.T) {
	t.Parallel()
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tt := range []struct {
		image    string
		expected string
	}{
		{image: "ghcr.io/team/agent:v1.2.3", expected: "offline.example/team/agent:v1.2.3"},
		{image: "registry.example:5443/team/agent:v1.2.3", expected: "offline.example/team/agent:v1.2.3"},
		{image: "registry.example:5443/team/agent", expected: "offline.example/team/agent:latest"},
		{image: "nginx", expected: "offline.example/library/nginx:latest"},
		{image: "nginx@" + digest, expected: "offline.example/library/nginx@" + digest},
		{image: "nginx:1.27@" + digest, expected: "offline.example/library/nginx@" + digest},
		{image: ""},
		{image: "nginx:"},
		{image: "https://ghcr.io/team/agent:v1.2.3"},
		{image: "nginx@sha256:invalid"},
	} {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()
			result, err := Apply(t.Context(), `{{ printf "offline.example/%s%s" (imageRepository .Values.image) (imageTagOrDigest .Values.image) }}`, NewObjects(value.Values{"image": tt.image}))
			if tt.expected == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
