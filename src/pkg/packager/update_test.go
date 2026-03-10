// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"testing"

	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestUpdateNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		zarfPackage        v1alpha1.ZarfPackage
		imageScans         []ComponentImageScan
		imageArchivesScans []ImageArchivesScan
		want               bool
	}{
		{
			name: "equal images in components and images scans",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd",
						Images: []string{
							"docker.io/library/redis:7.0.15-alpine",
							"quay.io/argoproj/argocd:v2.9.6",
							"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.sig",
							"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.att",
						},
					},
					{
						Name: "podinfo",
						Images: []string{
							"ghcr.io/stefanprodan/podinfo:6.4.0",
						},
					},
				},
			},
			imageScans: []ComponentImageScan{
				{
					ComponentName: "podinfo",
					Matches: []string{
						"ghcr.io/stefanprodan/podinfo:6.4.0",
					},
				},
				{

					ComponentName: "argocd",
					Matches: []string{
						"docker.io/library/redis:7.0.15-alpine",
						"quay.io/argoproj/argocd:v2.9.6",
					},
					CosignArtifacts: []string{
						"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.sig",
						"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.att",
					},
				},
			},
			want: false,
		},
		{
			name: "new image tags found",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd",
						Images: []string{
							"docker.io/library/redis:7.0.14-alpine",
							"quay.io/argoproj/argocd:v2.8.6",
						},
					},
				},
			},
			imageScans: []ComponentImageScan{
				{

					ComponentName: "argocd",
					Matches: []string{
						"docker.io/library/redis:7.0.15-alpine",
						"quay.io/argoproj/argocd:v2.9.6",
					},
				},
			},
			want: true,
		},
		{
			name: "images in components but not in image scans",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd",
						Images: []string{
							"docker.io/library/redis:7.0.14-alpine",
							"quay.io/argoproj/argocd:v2.8.6",
						},
					},
				},
			},
			imageScans: []ComponentImageScan{
				{

					ComponentName: "argocd",
					Matches: []string{
						"docker.io/library/redis:7.0.14-alpine",
					},
				},
			},
			want: true,
		},
		{
			name: "images in images scans but not in components",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd",
						Images: []string{
							"quay.io/argoproj/argocd:v2.8.6",
						},
					},
				},
			},
			imageScans: []ComponentImageScan{
				{

					ComponentName: "argocd",
					Matches: []string{
						"docker.io/library/redis:7.0.14-alpine",
						"quay.io/argoproj/argocd:v2.8.6",
					},
				},
			},
			want: true,
		},
		{
			name: "equal images in image archives components and image archives scans",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd-archive",
						ImageArchives: []v1alpha1.ImageArchive{
							{
								Images: []string{
									"docker.io/library/redis:7.0.15-alpine",
									"quay.io/argoproj/argocd:v2.9.6",
									"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.sig",
									"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.att",
								},
							},
						},
					},
				},
			},
			imageArchivesScans: []ImageArchivesScan{
				{
					ComponentName: "argocd-archive",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Images: []string{
								"docker.io/library/redis:7.0.15-alpine",
								"quay.io/argoproj/argocd:v2.9.6",
								"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.sig",
								"quay.io/argoproj/argocd:sha256-2dafd800fb617ba5b16ae429e388ca140f66f88171463d23d158b372bb2fae08.att",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "new archive image tags found",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd-archive",
						ImageArchives: []v1alpha1.ImageArchive{
							{
								Images: []string{
									"docker.io/library/redis:7.0.14-alpine",
								},
							},
						},
					},
				},
			},
			imageArchivesScans: []ImageArchivesScan{
				{
					ComponentName: "argocd-archive",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Images: []string{
								"docker.io/library/redis:7.0.15-alpine",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "image in components but not in archive scans",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd-archive",
						ImageArchives: []v1alpha1.ImageArchive{
							{
								Images: []string{
									"docker.io/library/redis:7.0.14-alpine",
									"quay.io/argoproj/argocd:v2.9.6",
								},
							},
						},
					},
				},
			},
			imageArchivesScans: []ImageArchivesScan{
				{
					ComponentName: "argocd-archive",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Images: []string{
								"docker.io/library/redis:7.0.14-alpine",
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "image in archive scans but not in components",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{
						Name: "argocd-archive",
						ImageArchives: []v1alpha1.ImageArchive{
							{
								Images: []string{
									"docker.io/library/redis:7.0.14-alpine",
								},
							},
						},
					},
				},
			},
			imageArchivesScans: []ImageArchivesScan{
				{
					ComponentName: "argocd-archive",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Images: []string{
								"docker.io/library/redis:7.0.14-alpine",
								"quay.io/argoproj/argocd:v2.9.6",
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := updateNeeded(tt.zarfPackage, tt.imageScans, tt.imageArchivesScans)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCreateUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		zarfPackage        v1alpha1.ZarfPackage
		imageScans         []ComponentImageScan
		imageArchivesScans []ImageArchivesScan
		inputYAML          string
		outputYAML         string
		wantErr            bool
	}{
		{
			name: "updates multiple components with all artifact types and preserves yaml structure",
			zarfPackage: v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{
					{Name: "flux"},
					{Name: "podinfo"},
					{Name: "flux-automation-controller-archive"},
				},
			},
			imageScans: []ComponentImageScan{
				{
					ComponentName: "flux",
					Matches: []string{
						"ghcr.io/fluxcd/helm-controller:v1.1.0",
					},
					CosignArtifacts: []string{
						"ghcr.io/fluxcd/helm-controller:sha256-4c75ca6c24ceb1f1bd7e935d9287a93e4f925c512f206763ec5a47de3ef3ff48.sig",
						"ghcr.io/fluxcd/helm-controller:sha256-4c75ca6c24ceb1f1bd7e935d9287a93e4f925c512f206763ec5a47de3ef3ff48.att",
						"ghcr.io/fluxcd/image-automation-controller:sha256-5b6c2e97055cfe69fe8996f48b53db039c136210dbc98c5631864a9e573d0e20.sig",
						"ghcr.io/fluxcd/image-automation-controller:sha256-5b6c2e97055cfe69fe8996f48b53db039c136210dbc98c5631864a9e573d0e20.att",
					},
				},

				{
					ComponentName: "podinfo",
					Matches:       []string{"ghcr.io/stefanprodan/podinfo:6.4.0"},
				},
			},
			imageArchivesScans: []ImageArchivesScan{
				{
					ComponentName: "flux-automation-controller-archive",
					ImageArchives: []v1alpha1.ImageArchive{
						{
							Path: "automation-controller.tar",
							Images: []string{
								"ghcr.io/fluxcd/image-automation-controller:v0.39.0",
							},
						},
					},
				},
			},
			inputYAML: `# Package metadata
metadata:
  name: test-package

components:
  # Flux component
  - name: flux
    description: Flux
    images:
      - ghcr.io/fluxcd/helm-controller:v1.0.0
      - ghcr.io/fluxcd/image-automation-controller:v0.38.0
  - name: podinfo
    images:
      - postgres:12
  - name: flux-automation-controller-archive
    imageArchives:
      - path: automation-controller.tar
`,
			outputYAML: `# Package metadata
metadata:
  name: test-package

components:
  # Flux component
  - name: flux
    description: Flux
    images:
      - ghcr.io/fluxcd/helm-controller:v1.1.0
      - ghcr.io/fluxcd/helm-controller:sha256-4c75ca6c24ceb1f1bd7e935d9287a93e4f925c512f206763ec5a47de3ef3ff48.sig
      - ghcr.io/fluxcd/helm-controller:sha256-4c75ca6c24ceb1f1bd7e935d9287a93e4f925c512f206763ec5a47de3ef3ff48.att
      - ghcr.io/fluxcd/image-automation-controller:sha256-5b6c2e97055cfe69fe8996f48b53db039c136210dbc98c5631864a9e573d0e20.sig
      - ghcr.io/fluxcd/image-automation-controller:sha256-5b6c2e97055cfe69fe8996f48b53db039c136210dbc98c5631864a9e573d0e20.att
  - name: podinfo
    images:
      - ghcr.io/stefanprodan/podinfo:6.4.0
  - name: flux-automation-controller-archive
    imageArchives:
      - images:
          - ghcr.io/fluxcd/image-automation-controller:v0.39.0
        path: automation-controller.tar
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astFile, err := parser.ParseBytes([]byte(tt.inputYAML), parser.ParseComments)
			require.NoError(t, err)

			got, err := createUpdate(tt.zarfPackage, tt.imageScans, tt.imageArchivesScans, astFile)

			require.NoError(t, err)
			require.Equal(t, tt.outputYAML, got)
		})
	}
}
