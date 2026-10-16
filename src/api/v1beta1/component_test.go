// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImageByID(t *testing.T) {
	t.Parallel()

	const digest = "sha256:594ceea76b01c592858f803f9ff4d2cb40542cae2060410b2c95f75907d659e1"
	tests := []struct {
		name       string
		ref        string
		repository string
		tag        string
		digest     string
	}{
		{name: "tag", ref: "docker.io/library/nginx:1.29.2", repository: "library/nginx", tag: "1.29.2"},
		{name: "digest", ref: "docker.io/library/nginx@" + digest, repository: "library/nginx", digest: digest},
		{name: "tag and digest", ref: "docker.io/library/nginx:1.29.2@" + digest, repository: "library/nginx", tag: "1.29.2", digest: digest},
		{name: "no tag or digest", ref: "nginx", repository: "library/nginx"},
		{name: "registry port is not a tag", ref: "localhost:5000/nginx", repository: "nginx"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			component := Component{
				Name: "example",
				ComponentSpec: ComponentSpec{Images: []Image{
					{Name: "busybox:latest"},
					{ID: "web", Name: tt.ref},
				}},
			}
			got, err := component.GetImageByID("web")
			require.NoError(t, err)
			require.Equal(t, ImageReference{Reference: tt.ref, Repository: tt.repository, Tag: tt.tag, Digest: tt.digest}, got)
		})
	}

	component := Component{
		Name: "example",
		ComponentSpec: ComponentSpec{Images: []Image{
			{Name: "busybox:latest"},
			{ID: "web", Name: "nginx:1.29.2"},
		}},
	}
	for _, id := range []string{"missing", "", "nginx:1.29.2"} {
		t.Run("rejects ID "+id, func(t *testing.T) {
			_, err := component.GetImageByID(id)
			require.ErrorContains(t, err, "no image with ID")
			require.ErrorContains(t, err, `component "example"`)
		})
	}

	t.Run("component without images", func(t *testing.T) {
		_, err := (Component{Name: "empty"}).GetImageByID("web")
		require.EqualError(t, err, `no image with ID "web" in component "empty"`)
	})

	t.Run("invalid reference", func(t *testing.T) {
		component := Component{
			Name:          "example",
			ComponentSpec: ComponentSpec{Images: []Image{{ID: "web", Name: "nginx:invalid tag"}}},
		}
		_, err := component.GetImageByID("web")
		require.ErrorContains(t, err, `invalid image reference "nginx:invalid tag"`)
		require.ErrorContains(t, err, `ID "web" in component "example"`)
	})
}

func TestGetImages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component Component
		expected  []string
	}{
		{
			name: "no images",
			component: Component{
				Name: "test-component",
			},
			expected: []string{},
		},
		{
			name: "only Images field",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					Images: []Image{
						{Name: "docker.io/library/nginx:latest"},
						{Name: "ghcr.io/zarf-dev/zarf:v0.32.6"},
					},
				},
			},
			expected: []string{
				"docker.io/library/nginx:latest",
				"ghcr.io/zarf-dev/zarf:v0.32.6",
			},
		},
		{
			name: "only ImageArchives with images",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					ImageArchives: []ImageArchive{
						{
							Path: "/tmp/images.tar",
							Images: []string{
								"docker.io/library/redis:latest",
								"docker.io/library/postgres:14",
							},
						},
					},
				},
			},
			expected: []string{
				"docker.io/library/redis:latest",
				"docker.io/library/postgres:14",
			},
		},
		{
			name: "both Images and ImageArchives",
			component: Component{
				Name: "test-component",
				ComponentSpec: ComponentSpec{
					Images: []Image{
						{Name: "docker.io/library/nginx:latest"},
					},
					ImageArchives: []ImageArchive{
						{
							Path: "/tmp/images1.tar",
							Images: []string{
								"docker.io/library/redis:latest",
							},
						},
						{
							Path: "/tmp/images2.tar",
							Images: []string{
								"docker.io/library/postgres:14",
								"ghcr.io/zarf-dev/zarf:v0.32.6",
							},
						},
					},
				},
			},
			expected: []string{
				"docker.io/library/nginx:latest",
				"docker.io/library/redis:latest",
				"docker.io/library/postgres:14",
				"ghcr.io/zarf-dev/zarf:v0.32.6",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.component.GetImages()
			require.Equal(t, tt.expected, result)
		})
	}
}
