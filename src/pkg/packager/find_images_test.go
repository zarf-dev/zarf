// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestFindImages(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	_ = feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}) //nolint:errcheck

	tests := []struct {
		name           string
		packagePath    string
		opts           FindImagesOptions
		expectedErr    string
		expectedImages []ComponentImageScan
	}{
		{
			name:        "agent deployment",
			packagePath: "./testdata/find-images/agent",
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
					},
					CosignArtifacts: []string{
						"ghcr.io/zarf-dev/zarf/agent:sha256-f8b1c2f99349516ae1bd0711a19697abcc41555076b0ae90f1a70ca6b50dcbd8.sig",
					},
				},
			},
		},
		{
			name:        "helm chart excludes test resources by default",
			packagePath: "./testdata/find-images/helm-chart",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"docker.io/library/nginx:1.16.0",
						"docker.io/library/alpine:latest",
					},
				},
			},
		},
		{
			name:        "kustomization",
			packagePath: "./testdata/find-images/kustomize",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
					},
				},
			},
		},
		{
			name:        "valid-image-uri",
			packagePath: "./testdata/find-images/valid-image-uri",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"10.0.0.1:443/zarf-dev/zarf/agent:v0.38.1",
						"docker.io/library/alpine:latest",
						"docker.io/library/foo_bar:latest",
						"foo.com:8080/bar:1.2.3",
						"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
						"registry.io/foo/project--id.module--name.ver---sion--name:latest",
						"xn--7o8h.com/myimage:9.8.7",
					},
				},
			},
		},
		{
			name:        "image not found",
			packagePath: "./testdata/find-images/agent",
			opts: FindImagesOptions{
				Why: "foobar",
			},
			expectedErr: "image foobar not found in any charts or manifests",
		},
		{
			name:        "invalid helm repository",
			packagePath: "./testdata/find-images/invalid-helm-repo",
			opts: FindImagesOptions{
				RepoHelmChartPath: "test",
			},
			expectedErr: "cannot convert the Git repository https://github.com/zarf-dev/zarf-public-test.git to a Helm chart without a version tag",
		},
		{
			name:        "validate repo Helm Chart ",
			packagePath: "./testdata/find-images/repo-chart-path",
			opts: FindImagesOptions{
				RepoHelmChartPath:   "charts/podinfo",
				KubeVersionOverride: "1.24.0-0",
				SkipCosign:          true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"docker.io/curlimages/curl:7.69.0",
						"docker.io/giantswarm/tiny-tools:latest",
						"docker.io/stefanprodan/grpc_health_probe:v0.3.0",
						"ghcr.io/stefanprodan/podinfo:6.4.0",
					},
				},
			},
		},
		{
			name:        "invalid manifest yaml",
			packagePath: "./testdata/find-images/invalid-manifest-yaml",
			opts: FindImagesOptions{
				SkipCosign: true,
				Why:        "foobar",
			},
			expectedErr: "failed to unmarshal manifest: error converting YAML to JSON: yaml: line 12: could not find expected ':'",
		},
		{
			name:        "ocirepo",
			packagePath: "./testdata/find-images/flux-oci-repo",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"ghcr.io/stefanprodan/manifests/podinfo:6.4.1",
						"ghcr.io/stefanprodan/manifests/podinfo@sha256:fc60d367cc05bedae04d6030e270daa89c3d82fa18b1a155314102b2fca39652",
					},
				},
			},
		},
		{
			name:        "fuzzy",
			packagePath: "./testdata/find-images/fuzzy",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches:       []string{},
					PotentialMatches: []string{
						"quay.io/cephcsi/cephcsi:v3.14.1",
						"quay.io/csiaddons/k8s-sidecar:v0.12.0",
						"registry.k8s.io/sig-storage/csi-attacher:v4.8.1",
						"registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.13.0",
						"registry.k8s.io/sig-storage/csi-provisioner:v5.2.0",
						"registry.k8s.io/sig-storage/csi-resizer:v1.13.2",
						"registry.k8s.io/sig-storage/csi-snapshotter:v8.2.1",
					},
				},
				{
					ComponentName: "underscores",
					Matches:       []string{},
					PotentialMatches: []string{
						"docker.io/percona/mongodb_exporter:0.47.1",
						"docker.io/library/alpine:3.23",
					},
				},
			},
		},
		{
			name:        "values from package definition",
			packagePath: "./testdata/find-images/values",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"docker.io/library/nginx:1.25.0",
					},
				},
			},
		},
		{
			name:        "values from options",
			packagePath: "./testdata/find-images/values-options",
			opts: FindImagesOptions{
				SkipCosign: true,
				Values: value.Values{
					"config": map[string]any{
						"tag": "1.24.0",
					},
				},
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"docker.io/library/nginx:1.24.0",
					},
				},
			},
		},
		{
			name:        "values from options override package definition",
			packagePath: "./testdata/find-images/values",
			opts: FindImagesOptions{
				SkipCosign: true,
				Values: value.Values{
					"config": map[string]any{
						"tag": "2.0.0",
					},
				},
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"docker.io/library/nginx:2.0.0",
					},
				},
			},
		},
		{
			name:        "pod volume image",
			packagePath: "./testdata/find-images/pod-volume-image",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"ghcr.io/zarf-dev/zarf/agent:v0.68.1",
						"quay.io/almalinuxorg/10-minimal:10",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imagesScans, err := FindImages(ctx, tt.packagePath, tt.opts)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, tt.expectedImages, len(imagesScans))
			for i, expected := range tt.expectedImages {
				require.Equal(t, expected.ComponentName, imagesScans[i].ComponentName)
				require.ElementsMatch(t, expected.Matches, imagesScans[i].Matches)
				require.ElementsMatch(t, expected.PotentialMatches, imagesScans[i].PotentialMatches)
				require.ElementsMatch(t, expected.CosignArtifacts, imagesScans[i].CosignArtifacts)
				require.ElementsMatch(t, expected.WhyResources, imagesScans[i].WhyResources)
			}
		})
	}
}

func TestFindImagesFromRepositoryHelmChartWithBranchRef(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	repoURL := createRepositoryHelmChart(t)
	packageDir := t.TempDir()
	packageDefinition := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: repository-chart
components:
  - name: baseline
    repositories:
      - url: %s
        ref:
          branch: main
`, repoURL)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.yaml"), []byte(packageDefinition), 0o600))

	images, err := FindImages(ctx, packageDir, FindImagesOptions{
		RepoHelmChartPath: "/",
		SkipCosign:        true,
	})
	require.NoError(t, err)
	require.Equal(t, []ComponentImageScan{{
		ComponentName: "baseline",
		Matches:       []string{"docker.io/library/nginx:1.25.0"},
	}}, images)
}

func createRepositoryHelmChart(t *testing.T) string {
	t.Helper()

	repositoryDir := t.TempDir()
	repository, err := git.PlainInitWithOptions(repositoryDir, &git.PlainInitOptions{
		InitOptions: git.InitOptions{DefaultBranch: plumbing.Main},
	})
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(repositoryDir, "templates"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(repositoryDir, "Chart.yaml"), []byte("apiVersion: v2\nname: repository-chart\nversion: 0.1.0\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(repositoryDir, "templates", "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: repository-chart
spec:
  selector:
    matchLabels:
      app: repository-chart
  template:
    metadata:
      labels:
        app: repository-chart
    spec:
      containers:
        - name: app
          image: nginx:1.25.0
`), 0o600))

	worktree, err := repository.Worktree()
	require.NoError(t, err)
	require.NoError(t, worktree.AddGlob("."))
	_, err = worktree.Commit("add chart", &git.CommitOptions{Author: &object.Signature{
		Name:  "Zarf Test",
		Email: "zarf@example.com",
		When:  time.Unix(0, 0),
	}})
	require.NoError(t, err)

	repositoryURLPath := filepath.ToSlash(repositoryDir)
	if !strings.HasPrefix(repositoryURLPath, "/") {
		repositoryURLPath = "/" + repositoryURLPath
	}
	return "file://" + repositoryURLPath
}

func TestFindDefinitionImages(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	tests := []struct {
		name                           string
		packagePath                    string
		expectedErr                    string
		expectedDefinitionImageResults []DefinitionImageResult
	}{
		{
			name:        "no image archives",
			packagePath: "./testdata/find-images/helm-chart/",
			expectedDefinitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "baseline",
						Matches: []string{
							"docker.io/library/alpine:latest",
							"docker.io/library/nginx:1.16.0",
						},
					},
				},
			},
		},
		{
			name:        "images found in archives",
			packagePath: "./testdata/find-images/image-archives/",
			expectedDefinitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "manifest-referencing-image-in-archive",
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "manifest-referencing-image-not-in-archive",
						Matches: []string{
							"docker.io/library/alpine:latest",
						},
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "image-archive-component",
					},
					ImageArchives: []api.ImageArchive{
						{
							Images: []string{
								"docker.io/library/scratch:latest",
							},
						},
					},
				},
			},
		},
		{
			name:        "images found in different archives",
			packagePath: "./testdata/find-images/multiple-image-archives/",
			expectedDefinitionImageResults: []DefinitionImageResult{
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "manifest-referencing-image-in-archive",
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "manifest-referencing-scratch-other-image-in-archive",
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "manifest-referencing-image-not-in-archive",
						Matches: []string{
							"docker.io/library/alpine:latest",
						},
					},
				},
				{
					ComponentImageScan: ComponentImageScan{
						ComponentName: "image-archive-component",
					},
					ImageArchives: []api.ImageArchive{
						{
							Images: []string{
								"docker.io/library/scratch:latest",
							},
						},
						{
							Images: []string{
								"docker.io/library/scratch:other",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := FindImagesOptions{}
			definitionImageResults, err := FindDefinitionImages(ctx, tt.packagePath, opts)

			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, tt.expectedDefinitionImageResults, len(definitionImageResults))

			for i, expected := range tt.expectedDefinitionImageResults {
				require.Equal(t, expected.ComponentName, definitionImageResults[i].ComponentName)
				require.ElementsMatch(t, expected.Matches, definitionImageResults[i].Matches)
				for j, expectedArchive := range expected.ImageArchives {
					require.ElementsMatch(t, expectedArchive.Images, definitionImageResults[i].ImageArchives[j].Images)
				}
			}
		})
	}
}

func TestFindImagesWhyExcludesHelmTestResources(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	packagePath := "./testdata/find-images/helm-chart"

	_, err := FindImages(ctx, packagePath, FindImagesOptions{Why: "busybox", SkipCosign: true})
	require.EqualError(t, err, "image busybox not found in any charts or manifests")
}

func TestBuildImageMap(t *testing.T) {
	t.Parallel()

	podSpec := corev1.PodSpec{
		InitContainers: []corev1.Container{
			{
				Image: "init-image",
			},
			{
				Image: "duplicate-image",
			},
		},
		Containers: []corev1.Container{
			{
				Image: "container-image",
			},
			{
				Image: "alpine:latest",
			},
		},
		EphemeralContainers: []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Image: "ephemeral-image",
				},
			},
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Image: "duplicate-image",
				},
			},
		},
	}
	imgMap := appendToImageMap(map[string]bool{}, podSpec)
	expectedImgMap := map[string]bool{
		"init-image":      true,
		"duplicate-image": true,
		"container-image": true,
		"alpine:latest":   true,
		"ephemeral-image": true,
	}
	require.Equal(t, expectedImgMap, imgMap)
}

func TestGetSortedImages(t *testing.T) {
	t.Parallel()

	matchedImages := map[string]bool{
		"C": true,
		"A": true,
		"E": true,
		"D": true,
	}
	maybeImages := map[string]bool{
		"Z": true,
		"A": true,
		"B": true,
	}
	sortedMatchedImages, sortedMaybeImages := getSortedImages(matchedImages, maybeImages)
	expectedSortedMatchedImages := []string{"A", "C", "D", "E"}
	require.Equal(t, expectedSortedMatchedImages, sortedMatchedImages)
	expectedSortedMaybeImages := []string{"B", "Z"}
	require.Equal(t, expectedSortedMaybeImages, sortedMaybeImages)
}
