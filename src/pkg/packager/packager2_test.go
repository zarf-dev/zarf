// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package packager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestOverridePackageNamespace(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name        string
		pkg         v1alpha1.ZarfPackage
		namespace   string
		expectedErr string
	}{
		{
			name: "override namespace",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
					},
				},
			},
			namespace: "test-override",
		},
		{
			name: "multiple namespaces",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
							{
								Name:      "test-2",
								Namespace: "test-2",
							},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "package contains 2 unique namespaces, cannot override namespace",
		},
		{
			name: "init package namespace override",
			pkg: v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Charts: []v1alpha1.ZarfChart{
							{
								Name:      "test",
								Namespace: "test",
							},
						},
					},
				},
			},
			namespace:   "test-override",
			expectedErr: "package kind is not a ZarfPackageConfig, cannot override namespace",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := OverridePackageNamespace(tc.pkg, tc.namespace)
			if tc.expectedErr == "" {
				require.NoError(t, err)
				validateNamespaceUpdates(t, tc.pkg, tc.namespace)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}

func validateNamespaceUpdates(t *testing.T, pkg v1alpha1.ZarfPackage, namespace string) {
	t.Helper()
	for _, component := range pkg.Components {
		for _, chart := range component.Charts {
			require.Equal(t, chart.Namespace, namespace)
		}
		for _, manifest := range component.Manifests {
			require.Equal(t, manifest.Namespace, namespace)
		}
	}
}
