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

	maxSeconds := 60
	maxRetries := 10

	tests := []struct {
		name   string
		oldPkg v1alpha1.ZarfPackage
		newPkg ZarfPackage
	}{
		{
			name: "test",
			oldPkg: v1alpha1.ZarfPackage{
				APIVersion: v1alpha1.APIVersion,
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
						Name: "manifests",
						Manifests: []v1alpha1.ZarfManifest{
							{
								NoWait: true,
							},
							{
								NoWait: false,
							},
						},
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
										MaxTotalSeconds: &maxSeconds,
										MaxRetries:      &maxRetries,
									},
								},
								After: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &maxSeconds,
										MaxRetries:      &maxRetries,
									},
									{
										MaxTotalSeconds: &maxSeconds,
										MaxRetries:      &maxRetries,
									},
								},
								OnSuccess: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &maxSeconds,
										MaxRetries:      &maxRetries,
									},
								},
								OnFailure: []v1alpha1.ZarfComponentAction{
									{
										MaxTotalSeconds: &maxSeconds,
										MaxRetries:      &maxRetries,
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
								NoWait:   true,
							},
							{
								URL:     "https://example.com/chart.git",
								GitPath: "path/to/chart2",
								NoWait:  false,
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
				APIVersion: APIVersion,
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
						Name:     "manifests",
						Optional: helpers.BoolPtr(true),
						Manifests: []ZarfManifest{
							{
								Wait: helpers.BoolPtr(false),
							},
							{
								Wait: helpers.BoolPtr(true),
							},
						},
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
										Retries: 10,
									},
								},
								After: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 10,
									},
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 10,
									},
								},
								OnSuccess: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 10,
									},
								},
								OnFailure: []ZarfComponentAction{
									{
										Timeout: &v1.Duration{Duration: time.Duration(time.Second * 60)},
										Retries: 10,
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
								Wait: helpers.BoolPtr(false),
								Helm: HelmRepoSource{
									URL:      "https://example.com/chart",
									RepoName: "repo1",
								},
							},
							{
								Wait: helpers.BoolPtr(true),
								Git: GitRepoSource{
									URL:  "https://example.com/chart.git",
									Path: "path/to/chart2",
								},
							},
							{
								Wait: helpers.BoolPtr(true),
								OCI: OCISource{
									URL: "oci://example.com/chart",
								},
							},
							{
								Wait: helpers.BoolPtr(true),
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
