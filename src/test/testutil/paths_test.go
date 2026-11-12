// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestChecksumZarfYAMLContentV1Beta1(t *testing.T) {
	t.Parallel()

	pkg1 := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: "test-pkg"},
		Build: v1beta1.BuildData{
			User:      "alice",
			Timestamp: "2026-01-01T00:00:00Z",
			Hostname:  "host-a",
			Version:   "v1.0.0",
		},
	}

	pkg2 := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: "test-pkg"},
		Build: v1beta1.BuildData{
			User:      "bob",
			Timestamp: "2026-10-31T12:00:00Z",
			Hostname:  "host-b",
			Version:   "v2.0.0",
		},
	}

	sum1 := ChecksumZarfYAMLContentV1Beta1(t, pkg1)
	sum2 := ChecksumZarfYAMLContentV1Beta1(t, pkg2)

	require.NotEmpty(t, sum1)
	require.Equal(t, sum1, sum2, "checksum must be deterministic and ignore BuildData")
}

func TestRequireNoBackslashInPackagePathsV1Beta1(t *testing.T) {
	t.Parallel()

	t.Run("passes on valid forward-slash paths", func(t *testing.T) {
		t.Parallel()

		dir := "scripts"
		pkg := v1beta1.Package{
			Values: v1beta1.Values{
				Files:  []string{"values/base.yaml", "values/override.yaml"},
				Schema: "schema/values.schema.json",
			},
			Components: []v1beta1.Component{
				{
					Name: "comp",
					ComponentSpec: v1beta1.ComponentSpec{
						Files: []v1beta1.File{
							{Source: "files/app.tar.gz"},
						},
						ImageArchives: []v1beta1.ImageArchive{
							{Path: "images/archive.tar"},
						},
						Charts: []v1beta1.Chart{
							{
								Name:  "my-chart",
								Local: &v1beta1.LocalSource{Path: "charts/my-chart"},
								ValuesFiles: []v1beta1.ValuesFile{
									{Path: "charts/values.yaml"},
								},
							},
						},
						Manifests: []v1beta1.Manifest{
							{
								Name:  "manifest",
								Files: []string{"manifests/deploy.yaml"},
								Kustomize: v1beta1.KustomizeManifest{
									Files: []string{"manifests/kustomization"},
								},
							},
						},
						Import: v1beta1.ComponentImport{
							Local: []v1beta1.ComponentImportLocal{
								{Path: "components/imported.yaml"},
							},
						},
						Actions: v1beta1.ComponentActions{
							OnCreate: v1beta1.ComponentActionSet{
								Defaults: &v1beta1.ComponentActionDefaults{Dir: "actions/dir"},
								Before:   []v1beta1.ComponentAction{{Dir: &dir}},
							},
							OnDeploy: v1beta1.ComponentActionSet{
								OnSuccess: []v1beta1.ComponentAction{{Dir: &dir}},
							},
							OnRemove: v1beta1.ComponentActionSet{
								OnFailure: []v1beta1.ComponentAction{{Dir: &dir}},
							},
						},
					},
				},
			},
		}

		RequireNoBackslashInPackagePathsV1Beta1(t, pkg)
	})
}
