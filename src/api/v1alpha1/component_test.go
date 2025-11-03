// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component ZarfComponent
		expected  []string
	}{
		{
			name: "no images",
			component: ZarfComponent{
				Name: "test-component",
			},
			expected: []string{},
		},
		{
			name: "only Images field",
			component: ZarfComponent{
				Name: "test-component",
				Images: []string{
					"docker.io/library/nginx:latest",
					"ghcr.io/zarf-dev/zarf:v0.32.6",
				},
			},
			expected: []string{
				"docker.io/library/nginx:latest",
				"ghcr.io/zarf-dev/zarf:v0.32.6",
			},
		},
		{
			name: "only ImageTars with images",
			component: ZarfComponent{
				Name: "test-component",
				ImageTars: []ImageTar{
					{
						Path: "/tmp/images.tar",
						Images: []string{
							"docker.io/library/redis:latest",
							"docker.io/library/postgres:14",
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
			name: "both Images and ImageTars",
			component: ZarfComponent{
				Name: "test-component",
				Images: []string{
					"docker.io/library/nginx:latest",
				},
				ImageTars: []ImageTar{
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
