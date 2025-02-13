// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createPodAdmissionRequest(t *testing.T, op v1.Operation, pod *corev1.Pod) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestPodMutationWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	state := &types.ZarfState{RegistryInfo: types.RegistryInfo{Address: "127.0.0.1:31999"}}
	c := createTestClientWithZarfState(ctx, t, state)
	handler := admission.NewHandler().Serve(ctx, NewPodMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "pod with label should be mutated",
			admissionReq: createPodAdmissionRequest(t, v1.Create, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"should-be": "mutated"},
					Annotations: map[string]string{"should-be": "mutated"},
				},
				Spec: corev1.PodSpec{
					Containers:     []corev1.Container{{Name: "nginx", Image: "nginx"}},
					InitContainers: []corev1.Container{{Name: "different", Image: "busybox"}},
					EphemeralContainers: []corev1.EphemeralContainer{
						{
							EphemeralContainerCommon: corev1.EphemeralContainerCommon{
								Name:  "alpine",
								Image: "alpine",
							},
						},
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/imagePullSecrets",
					[]corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}},
				),
				operations.ReplacePatchOperation(
					"/spec/initContainers/0/image",
					"127.0.0.1:31999/library/busybox:latest-zarf-2140033595",
				),
				operations.ReplacePatchOperation(
					"/spec/ephemeralContainers/0/image",
					"127.0.0.1:31999/library/alpine:latest-zarf-1117969859",
				),
				operations.ReplacePatchOperation(
					"/spec/containers/0/image",
					"127.0.0.1:31999/library/nginx:latest-zarf-3793515731",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"zarf-agent": "patched",
						"should-be":  "mutated",
					},
				),
				operations.ReplacePatchOperation(
					"/metadata/annotations",
					map[string]string{
						"zarf.dev/original-image-nginx":     "nginx",
						"zarf.dev/original-image-alpine":    "alpine",
						"zarf.dev/original-image-different": "busybox",
						"should-be":                         "mutated",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "pod with zarf-agent patched label should not be mutated",
			admissionReq: createPodAdmissionRequest(t, v1.Create, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"zarf-agent": "patched"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Image: "nginx"}},
				},
			}),
			patch: nil,
			code:  http.StatusOK,
		},
		{
			name: "pod with no labels should not error",
			admissionReq: createPodAdmissionRequest(t, v1.Create, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/imagePullSecrets",
					[]corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}},
				),
				operations.ReplacePatchOperation(
					"/spec/containers/0/image",
					"127.0.0.1:31999/library/nginx:latest-zarf-3793515731",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{"zarf-agent": "patched"},
				),
				operations.ReplacePatchOperation(
					"/metadata/annotations",
					map[string]string{
						"zarf.dev/original-image-nginx": "nginx",
					},
				),
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
func TestGetImageAnnotationKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		containerName string
		expectedKey   string
	}{
		{
			containerName: "nginx",
			expectedKey:   "zarf.dev/original-image-nginx",
		},
		{
			containerName: "a-very-long-container-name-that-exceeds-sixty-three-characters",
			expectedKey:   "zarf.dev/original-image-a-very-long-container-name-that-exceeds-sixty-th",
		},
		{
			containerName: "remove-trailing-hyphen----",
			expectedKey:   "zarf.dev/original-image-remove-trailing-hyphen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.containerName, func(t *testing.T) {
			t.Parallel()
			key := getImageAnnotationKey(context.Background(), tt.containerName)
			require.Equal(t, tt.expectedKey, key)
		})
	}
}
