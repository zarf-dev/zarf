// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestApplyImageReferenceRepository(t *testing.T) {
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
			objects := NewObjects(nil).WithPackage(api.Package{
				APIVersion: v1beta1.APIVersion,
				Components: []api.Component{{
					Name:   "web",
					Images: []api.Image{{ID: "app", Name: tt.image}},
				}},
			})
			result, err := Apply(t.Context(), `{{ ((.Pkg.GetComponent "web").GetImageByID "app").Repository }}`, objects)
			if tt.expected == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
