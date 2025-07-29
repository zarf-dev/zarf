// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestFindImages(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	htp, err := utils.GetHtpasswdString("axol", "otl")
	require.NoError(t, err)

	address := testutil.SetupInMemoryRegistryWithAuth(ctx, t, 65000, htp)

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

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
			name:        "helm chart",
			packagePath: "./testdata/find-images/helm-chart",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					Matches: []string{
						"nginx:1.16.0",
						"busybox",
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
						"curlimages/curl:7.69.0",
						"giantswarm/tiny-tools",
						"stefanprodan/grpc_health_probe:v0.3.0",
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
			name:        "fuzzy-upstream",
			packagePath: "./testdata/find-images/fuzzy-upstream",
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
			},
		},
		{
			name:        "fuzzy-registry-auth",
			packagePath: "./testdata/find-images/fuzzy-registry-auth",
			opts: FindImagesOptions{
				SkipCosign: true,
			},
			expectedImages: []ComponentImageScan{
				{
					ComponentName: "baseline",
					PotentialMatches: []string{
						"registry1.dso.mil/ironbank/kiwigrid/k8s-sidecar:v1.12.0",
						"registry1.dso.mil/ironbank/opensource/ceph/ceph-csi:v3.14.1",
						"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/sig-storage/csi-attacher:v4.8.1",
						"registry1.dso.mil/ironbank/opensource/kubernetes-sigs/sig-storage/csi-provisioner:v5.2.0",
						fmt.Sprintf("%s/sig-storage/csi-snapshotter:v8.2.1", address),
						fmt.Sprintf("%s/sig-storage/csi-resizer:v1.13.2", address),
						fmt.Sprintf("%s/sig-storage/csi-node-driver-registrar:v2.13.0", address),
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
