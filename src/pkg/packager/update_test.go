// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestUpdateImagesV1Beta1PreservesAuthoredFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		manifest string
		target   api.ComponentTarget
		check    func(*testing.T, []byte)
	}{
		{
			name:     "package",
			manifest: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: example\ncomponents:\n  - name: app\n    import:\n      local:\n        - path: app.yaml\n    images:\n      - name: example.com/old:1\n        source: daemon\n",
			check: func(t *testing.T, b []byte) {
				var pkg v1beta1.Package
				require.NoError(t, yaml.Unmarshal(b, &pkg))
				require.Equal(t, []v1beta1.Image{{Name: "example.com/old:1", Source: "daemon"}, {Name: "example.com/new:1"}}, pkg.Components[0].Images)
				require.Equal(t, "app.yaml", pkg.Components[0].Import.Local[0].Path)
			},
		},
		{
			name:     "component config",
			manifest: "apiVersion: zarf.dev/v1beta1\nkind: ZarfComponentConfig\nmetadata:\n  name: app\ncomponent:\n  import:\n    local:\n      - path: app.yaml\n  images:\n    - name: example.com/old:1\n      source: daemon\n",
			check: func(t *testing.T, b []byte) {
				var config v1beta1.ComponentConfig
				require.NoError(t, yaml.Unmarshal(b, &config))
				require.Equal(t, []v1beta1.Image{{Name: "example.com/old:1", Source: "daemon"}, {Name: "example.com/new:1"}}, config.Component.Images)
				require.Equal(t, "app.yaml", config.Component.Import.Local[0].Path)
			},
		},
		{
			name:     "selects matching package variant",
			target:   api.ComponentTarget{Architecture: "amd64"},
			manifest: "apiVersion: zarf.dev/v1beta1\nkind: ZarfPackageConfig\nmetadata:\n  name: example\ncomponents:\n  - name: app\n    selector:\n      architecture: amd64\n    images:\n      - name: example.com/old:1\n        source: daemon\n  - name: app\n    selector:\n      architecture: arm64\n    images:\n      - name: example.com/arm:1\n      - name: example.com/arm-other:1\n",
			check: func(t *testing.T, b []byte) {
				var pkg v1beta1.Package
				require.NoError(t, yaml.Unmarshal(b, &pkg))
				require.Equal(t, []v1beta1.Image{{Name: "example.com/old:1", Source: "daemon"}, {Name: "example.com/new:1"}}, pkg.Components[0].Images)
				require.Equal(t, []v1beta1.Image{{Name: "example.com/arm:1"}, {Name: "example.com/arm-other:1"}}, pkg.Components[1].Images)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "definition.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.manifest), 0o600))
			result := DefinitionImageResult{ComponentImageScan: ComponentImageScan{ComponentName: "app", Matches: []string{"example.com/old:1", "example.com/new:1"}}, Target: tc.target}
			results := []DefinitionImageResult{result}
			require.NoError(t, UpdateImages(context.Background(), path, results))
			b, err := os.ReadFile(path)
			require.NoError(t, err)
			tc.check(t, b)
		})
	}
}

func TestImageUpdateNeeded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		zarfPackage            v1alpha1.ZarfPackage
		definitionImageResults []DefinitionImageResult
		want                   bool
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "podinfo",
						Matches: []string{
							"ghcr.io/stefanprodan/podinfo:6.4.0",
						},
					},
				},
				{
					ComponentImageScan: ComponentImageScan{

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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{

						ComponentName: "argocd",
						Matches: []string{
							"docker.io/library/redis:7.0.15-alpine",
							"quay.io/argoproj/argocd:v2.9.6",
						},
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd",
						Matches: []string{
							"docker.io/library/redis:7.0.14-alpine",
						},
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd",
						Matches: []string{
							"docker.io/library/redis:7.0.14-alpine",
							"quay.io/argoproj/argocd:v2.8.6",
						},
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd-archive",
					},
					ImageArchives: []api.ImageArchive{
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd-archive",
					},
					ImageArchives: []api.ImageArchive{
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd-archive",
					},
					ImageArchives: []api.ImageArchive{
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "argocd-archive",
					},
					ImageArchives: []api.ImageArchive{
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
			got := imageUpdateNeeded(tt.zarfPackage, tt.definitionImageResults)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCreateSchemaUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		zarfPackage    v1alpha1.ZarfPackage
		schemaFilename string
		inputYAML      string
		outputYAML     string
	}{
		{
			name:           "adds values.schema when no values key exists",
			zarfPackage:    v1alpha1.ZarfPackage{},
			schemaFilename: "values.schema.json",
			inputYAML: `metadata:
  name: test-package
`,
			outputYAML: `metadata:
  name: test-package
values:
  schema: values.schema.json
`,
		},
		{
			name: "adds schema under existing values while preserving files",
			zarfPackage: v1alpha1.ZarfPackage{
				Values: v1alpha1.ZarfValues{
					Files: []string{"values.yaml"},
				},
			},
			schemaFilename: "values.schema.json",
			inputYAML: `metadata:
  name: test-package
values:
  files:
    - values.yaml
`,
			outputYAML: `metadata:
  name: test-package
values:
  files:
    - values.yaml
  schema: values.schema.json
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astFile, err := parser.ParseBytes([]byte(tt.inputYAML), parser.ParseComments)
			require.NoError(t, err)

			err = createSchemaUpdate(tt.zarfPackage, tt.schemaFilename, astFile)

			require.NoError(t, err)
			require.Equal(t, tt.outputYAML, astFile.String())
		})
	}
}

func TestCreateImageUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		zarfPackage            v1alpha1.ZarfPackage
		definitionImageResults []DefinitionImageResult
		inputYAML              string
		outputYAML             string
		wantErr                bool
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
			definitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
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
				},

				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "podinfo",
						Matches:       []string{"ghcr.io/stefanprodan/podinfo:6.4.0"},
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "flux-automation-controller-archive",
					},
					ImageArchives: []api.ImageArchive{
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
      - path: automation-controller.tar
        images:
          - ghcr.io/fluxcd/image-automation-controller:v0.39.0
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astFile, err := parser.ParseBytes([]byte(tt.inputYAML), parser.ParseComments)
			require.NoError(t, err)

			err = createImageUpdate(tt.zarfPackage, tt.definitionImageResults, astFile)

			require.NoError(t, err)
			require.Equal(t, tt.outputYAML, astFile.String())
		})
	}
}
