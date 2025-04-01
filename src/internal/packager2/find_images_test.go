// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestFindImages(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	tests := []struct {
		name           string
		packagePath    string
		opts           FindImagesOptions
		expectedErr    string
		expectedImages map[string][]string
	}{
		{
			name:        "agent deployment",
			packagePath: "./testdata/find-images/agent",
			expectedImages: map[string][]string{
				"baseline": {
					"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
					"ghcr.io/zarf-dev/zarf/agent:sha256-f8b1c2f99349516ae1bd0711a19697abcc41555076b0ae90f1a70ca6b50dcbd8.sig",
				},
			},
		},
		{
			name:        "helm chart",
			packagePath: "./testdata/find-images/helm-chart",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: map[string][]string{
				"baseline": {
					"nginx:1.16.0",
					"busybox",
				},
			},
		},
		{
			name:        "kustomization",
			packagePath: "./testdata/find-images/kustomize",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: map[string][]string{
				"baseline": {
					"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
				},
			},
		},
		{
			name:        "valid-image-uri",
			packagePath: "./testdata/find-images/valid-image-uri",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: map[string][]string{
				"baseline": {
					"ghcr.io/zarf-dev/zarf/agent:v0.38.1",
					"10.0.0.1:443/zarf-dev/zarf/agent:v0.38.1",
					"alpine",
					"xn--7o8h.com/myimage:9.8.7",
					"registry.io/foo/project--id.module--name.ver---sion--name",
					"foo_bar:latest",
					"foo.com:8080/bar:1.2.3",
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
			name:        "invalid manifest yaml",
			packagePath: "./testdata/find-images/invalid-manifest-yaml",
			opts: FindImagesOptions{
				SkipCosign: true,
				Why:        "foobar",
			},
			expectedErr: "failed to unmarshal manifest: error converting YAML to JSON: yaml: line 12: could not find expected ':'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images, err := FindImages(ctx, tt.packagePath, tt.opts)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tt.expectedImages), len(images))
			for k, v := range tt.expectedImages {
				require.ElementsMatch(t, v, images[k])
			}
		})
	}
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
