// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 holds the definition of the v1beta1 Zarf Package
package v1beta1

import (
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslate(t *testing.T) {
	t.Parallel()

	value := 60

	tests := []struct {
		name   string
		oldPkg v1alpha1.ZarfPackage
		newPkg ZarfPackage
	}{
		{
			name: "test",
			oldPkg: v1alpha1.ZarfPackage{
				APIVersion: v1alpha1.ApiVersion,
				Kind:       v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Name:     "optional",
						Required: helpers.BoolPtr(false),
					},
					{
						Name:     "not-optional",
						Required: helpers.BoolPtr(true),
					},
					{
						Name: "actions",
						Actions: v1alpha1.ZarfComponentActions{
							OnCreate: v1alpha1.ZarfComponentActionSet{
								Defaults: v1alpha1.ZarfComponentActionDefaults{
									MaxTotalSeconds: 30,
									MaxRetries:      5,
								},
								Before: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &value,
										MaxRetries:      &value,
									},
								},
								After: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &value,
										MaxRetries:      &value,
									},
									{
										MaxTotalSeconds: &value,
										MaxRetries:      &value,
									},
								},
								OnSuccess: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &value,
										MaxRetries:      &value,
									},
								},
								OnFailure: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &value,
										MaxRetries:      &value,
									},
								},
							},
							OnDeploy: v1alpha1.ZarfComponentActionSet{
								Defaults: v1alpha1.ZarfComponentActionDefaults{
									MaxTotalSeconds: 30,
									MaxRetries:      5,
								},
							},
							OnRemove: v1alpha1.ZarfComponentActionSet{
								Defaults: v1alpha1.ZarfComponentActionDefaults{
									MaxTotalSeconds: 30,
									MaxRetries:      5,
								},
							},
						},
					},
					{
						Name: "helm-chart",
						Charts: []v1alpha1.ZarfChart{
							{
								URL:      "https://example.com/chart",
								RepoName: "repo1",
							},
							{
								URL:     "https://example.com/chart.git",
								GitPath: "path/to/chart2",
							},
							{
								URL: "oci://example.com/chart",
							},
							{
								LocalPath: "path/to/chart4",
							},
						},
					},
				},
			},
			newPkg: ZarfPackage{
				APIVersion: ApiVersion,
				Kind:       ZarfPackageConfig,
				Metadata: ZarfMetadata{
					Annotations: map[string]string{},
				},
				Components: []ZarfComponent{
					{
						Name:     "optional",
						Optional: helpers.BoolPtr(true),
					},
					{
						Name:     "not-optional",
						Optional: helpers.BoolPtr(false),
					},
					{
						Name:     "actions",
						Optional: helpers.BoolPtr(true),
						Actions: ZarfComponentActions{
							OnCreate: ZarfComponentActionSet{
								Defaults: ZarfComponentActionDefaults{
									Timeout: &v1.Duration{Duration: time.Duration(time.Second * 30)},
									Retries: 5,
								},
								Before: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 60,
									},
								},
								After: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 60,
									},
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 60,
									},
								},
								OnSuccess: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 60,
									},
								},
								OnFailure: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 60,
									},
								},
							},
							OnDeploy: ZarfComponentActionSet{
								Defaults: ZarfComponentActionDefaults{
									Timeout: &v1.Duration{Duration: time.Duration(time.Second * 30)},
									Retries: 5,
								},
							},
							OnRemove: ZarfComponentActionSet{
								Defaults: ZarfComponentActionDefaults{
									Timeout: &v1.Duration{Duration: time.Duration(time.Second * 30)},
									Retries: 5,
								},
							},
						},
					},
					{
						Name:     "helm-chart",
						Optional: helpers.BoolPtr(true),
						Charts: []ZarfChart{
							{
								Helm: HelmRepoSource{
									Url:      "https://example.com/chart",
									RepoName: "repo1",
								},
							},
							{
								Git: GitRepoSource{
									Url:  "https://example.com/chart.git",
									Path: "path/to/chart2",
								},
							},
							{
								OCI: OCISource{
									Url: "oci://example.com/chart",
								},
							},
							{
								Local: LocalRepoSource{
									Path: "path/to/chart4",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			translatedPkg, err := TranslateAlphaPackage(tc.oldPkg)
			require.NoError(t, err)
			require.Equal(t, tc.newPkg, translatedPkg)
		})
	}
}
