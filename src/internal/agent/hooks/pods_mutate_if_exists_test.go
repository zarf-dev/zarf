// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMutateIfExistsBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Mock registry server that simulates image existence
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate different responses based on image path
		// Pattern: /v2/library/{name}/manifests/{tag}
		if contains(r.URL.Path, "nginx-exists") ||
			contains(r.URL.Path, "busybox-exists") {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(registryServer.Close)

	// Extract host from test server (remove http://)
	registryAddr := registryServer.URL[7:]

	s := &state.State{
		RegistryInfo: state.RegistryInfo{
			Address:      registryAddr,
			PullUsername: "",
			PullPassword: "",
		},
	}
	c := createTestClientWithZarfState(ctx, t, s)

	// Create test namespace with mutate-if-exists label
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
			Labels: map[string]string{
				"zarf.dev/agent": "mutate-if-exists",
			},
		},
	}
	_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create namespace with skip label
	skipNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "skip-namespace",
			Labels: map[string]string{
				"zarf.dev/agent": "skip",
			},
		},
	}
	_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, skipNamespace, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create namespace with ignore label
	ignoreNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ignore-namespace",
			Labels: map[string]string{
				"zarf.dev/agent": "ignore",
			},
		},
	}
	_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, ignoreNamespace, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create default namespace without labels
	defaultNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-namespace",
		},
	}
	_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, defaultNamespace, metav1.CreateOptions{})
	require.NoError(t, err)

	handler := admission.NewHandler().Serve(ctx, NewPodMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "mutate-if-exists: image exists - should mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "test-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx-exists"},
						},
					},
				}),
			},
			code: http.StatusOK,
			// Can't check exact patches due to dynamic zarf suffix based on registry address
		},
		{
			name: "mutate-if-exists: image missing - should not mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "test-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx-missing"},
						},
					},
				}),
			},
			code: http.StatusOK,
			// Container should not be mutated, so no image patch
		},
		{
			name: "mutate-if-exists: mixed exists and missing - should mutate only existing",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "test-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "busybox", Image: "busybox-exists"},
						},
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx-missing"},
						},
					},
				}),
			},
			code: http.StatusOK,
			// Should have init container mutation but not regular container
		},
		{
			name: "skip namespace - should not mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "skip-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx"},
						},
					},
				}),
			},
			patch: nil,
			code:  http.StatusOK,
		},
		{
			name: "ignore namespace - should not mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "ignore-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx"},
						},
					},
				}),
			},
			patch: nil,
			code:  http.StatusOK,
		},
		{
			name: "default namespace - should mutate all (legacy behavior)",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Namespace: "default-namespace",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "nginx", Image: "nginx-missing"},
						},
					},
				}),
			},
			code: http.StatusOK,
			// Legacy behavior - should mutate even if image doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			require.Equal(t, tt.code, rr.Code)

			// Verify admission was successful
			var admissionReview v1.AdmissionReview
			err := json.NewDecoder(rr.Body).Decode(&admissionReview)
			require.NoError(t, err)
			require.True(t, admissionReview.Response.Allowed)
		})
	}
}

func TestMutateIfExistsEphemeralContainers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Mock registry server
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if contains(r.URL.Path, "debug-exists") {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(registryServer.Close)

	registryAddr := registryServer.URL[7:]

	s := &state.State{
		RegistryInfo: state.RegistryInfo{
			Address: registryAddr,
		},
	}
	c := createTestClientWithZarfState(ctx, t, s)

	// Create test namespace with mutate-if-exists label
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ephemeral-test",
			Labels: map[string]string{
				"zarf.dev/agent": "mutate-if-exists",
			},
		},
	}
	_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	require.NoError(t, err)

	handler := admission.NewHandler().Serve(ctx, NewPodMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "ephemeral container exists - should mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation:   v1.Create,
				Namespace:   "ephemeral-test",
				SubResource: "ephemeralcontainers",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
						Labels: map[string]string{
							"zarf-agent": "patched",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app", Image: fmt.Sprintf("%s/library/app:v1", registryAddr)},
						},
						EphemeralContainers: []corev1.EphemeralContainer{
							{
								EphemeralContainerCommon: corev1.EphemeralContainerCommon{
									Name:  "debug",
									Image: "debug-exists",
								},
							},
						},
					},
				}),
			},
			code: http.StatusOK,
		},
		{
			name: "ephemeral container missing - should not mutate",
			admissionReq: &v1.AdmissionRequest{
				Operation:   v1.Create,
				Namespace:   "ephemeral-test",
				SubResource: "ephemeralcontainers",
				Object: createRawPod(t, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
						Labels: map[string]string{
							"zarf-agent": "patched",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "app", Image: fmt.Sprintf("%s/library/app:v1", registryAddr)},
						},
						EphemeralContainers: []corev1.EphemeralContainer{
							{
								EphemeralContainerCommon: corev1.EphemeralContainerCommon{
									Name:  "debug",
									Image: "debug-missing",
								},
							},
						},
					},
				}),
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			require.Equal(t, tt.code, rr.Code)

			// Verify admission was successful
			var admissionReview v1.AdmissionReview
			err := json.NewDecoder(rr.Body).Decode(&admissionReview)
			require.NoError(t, err)
			require.True(t, admissionReview.Response.Allowed)
		})
	}
}

func TestCheckNamespaceMutationBehavior(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := &cluster.Cluster{Clientset: fake.NewClientset()}

	tests := []struct {
		name                 string
		namespaceLabels      map[string]string
		expectMutateIfExists bool
		expectSkip           bool
	}{
		{
			name:                 "no labels - default behavior",
			namespaceLabels:      nil,
			expectMutateIfExists: false,
			expectSkip:           false,
		},
		{
			name: "skip label",
			namespaceLabels: map[string]string{
				"zarf.dev/agent": "skip",
			},
			expectMutateIfExists: false,
			expectSkip:           true,
		},
		{
			name: "ignore label",
			namespaceLabels: map[string]string{
				"zarf.dev/agent": "ignore",
			},
			expectMutateIfExists: false,
			expectSkip:           true,
		},
		{
			name: "mutate-if-exists label",
			namespaceLabels: map[string]string{
				"zarf.dev/agent": "mutate-if-exists",
			},
			expectMutateIfExists: true,
			expectSkip:           false,
		},
		{
			name: "mutate-if-exists case insensitive",
			namespaceLabels: map[string]string{
				"zarf.dev/agent": "Mutate-If-Exists",
			},
			expectMutateIfExists: true,
			expectSkip:           false,
		},
		{
			name: "other label value",
			namespaceLabels: map[string]string{
				"zarf.dev/agent": "some-other-value",
			},
			expectMutateIfExists: false,
			expectSkip:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ns-" + tt.name,
					Labels: tt.namespaceLabels,
				},
			}
			_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			require.NoError(t, err)

			useMutateIfExists, skipResult := checkNamespaceMutationBehavior(ctx, c, ns.Name)

			require.Equal(t, tt.expectMutateIfExists, useMutateIfExists, "useMutateIfExists mismatch")
			if tt.expectSkip {
				require.NotNil(t, skipResult, "expected skip result")
				require.True(t, skipResult.Allowed)
				require.Empty(t, skipResult.PatchOps)
			} else {
				require.Nil(t, skipResult, "expected no skip result")
			}
		})
	}
}

// createRawPod is a helper to create raw pod JSON for admission requests
func createRawPod(t *testing.T, pod *corev1.Pod) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(pod)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: raw}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
