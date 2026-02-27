// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createArgoAppProjectAdmissionRequest(t *testing.T, op v1.Operation, argoAppProject *AppProject) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(argoAppProject)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestArgoAppProjectWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{
		GitServer: state.GitServerInfo{
			Address:      "https://git-server.com",
			PushUsername: "a-push-user",
		},
		RegistryInfo: state.RegistryInfo{
			Address: "127.0.0.1:31999",
		},
	}

	tests := []admissionTest{
		{
			name: "should be mutated",
			admissionReq: createArgoAppProjectAdmissionRequest(t, v1.Create, &AppProject{
				Spec: AppProjectSpec{
					SourceRepos: []string{
						"https://diff-git-server.com/cashews",
						"https://diff-git-server.com/almonds",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/sourceRepos/0",
					"https://git-server.com/a-push-user/cashews-580170494",
				),
				operations.ReplacePatchOperation(
					"/spec/sourceRepos/1",
					"https://git-server.com/a-push-user/almonds-640159520",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"zarf-agent": "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI repo",
			admissionReq: createArgoAppProjectAdmissionRequest(t, v1.Create, &AppProject{
				Spec: AppProjectSpec{
					SourceRepos: []string{
						"oci://ghcr.io/stefanprodan/charts/podinfo",
						"oci://registry-1.docker.io/dhpup/oci-edge",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/sourceRepos/0",
					"oci://127.0.0.1:31999/stefanprodan/charts/podinfo",
				),
				operations.ReplacePatchOperation(
					"/spec/sourceRepos/1",
					"oci://127.0.0.1:31999/dhpup/oci-edge",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"zarf-agent": "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI repo with internal service registry",
			admissionReq: createArgoAppProjectAdmissionRequest(t, v1.Create, &AppProject{
				Spec: AppProjectSpec{
					SourceRepos: []string{
						"oci://ghcr.io/stefanprodan/charts/podinfo",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/sourceRepos/0",
					"oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts/podinfo",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"zarf-agent": "patched",
					},
				),
			},
			svc: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zarf-docker-registry",
					Namespace: "zarf",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
					Ports: []corev1.ServicePort{
						{
							NodePort: 31999,
							Port:     5000,
						},
					},
					ClusterIP: "10.11.12.13",
				},
			},
			code: http.StatusOK,
		},
		{
			name: "should ignore unknown git URL",
			admissionReq: createArgoAppProjectAdmissionRequest(t, v1.Create, &AppProject{
				Spec: AppProjectSpec{
					SourceRepos: []string{"https://unknown-url"},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"zarf-agent": "patched",
					},
				),
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := createTestClientWithZarfState(ctx, t, s)
			handler := admission.NewHandler().Serve(ctx, NewAppProjectMutationHook(ctx, c))
			if tt.svc != nil {
				_, err := c.Clientset.CoreV1().Services("zarf").Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
