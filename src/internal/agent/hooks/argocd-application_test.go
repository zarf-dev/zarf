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

func createArgoAppAdmissionRequest(t *testing.T, op v1.Operation, argoApp *Application) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(argoApp)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestArgoAppWebhook(t *testing.T) {
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
	c := createTestClientWithZarfState(ctx, t, s)
	handler := admission.NewHandler().Serve(ctx, NewApplicationMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "should be mutated",
			admissionReq: createArgoAppAdmissionRequest(t, v1.Create, &Application{
				Spec: ApplicationSpec{
					Source: &ApplicationSource{RepoURL: "https://diff-git-server.com/peanuts"},
					Sources: []ApplicationSource{
						{
							RepoURL: "https://diff-git-server.com/cashews",
						},
						{
							RepoURL: "https://diff-git-server.com/almonds",
						},
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/source/repoURL",
					"https://git-server.com/a-push-user/peanuts-3883081014",
				),
				operations.ReplacePatchOperation(
					"/spec/sources/0/repoURL",
					"https://git-server.com/a-push-user/cashews-580170494",
				),
				operations.ReplacePatchOperation(
					"/spec/sources/1/repoURL",
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
			admissionReq: createArgoAppAdmissionRequest(t, v1.Create, &Application{
				Spec: ApplicationSpec{
					Source: &ApplicationSource{RepoURL: "oci://ghcr.io/stefanprodan/charts/podinfo"},
					Sources: []ApplicationSource{
						{
							RepoURL: "oci://ghcr.io/stefanprodan/manifests/podinfo",
						},
						{
							RepoURL: "https://diff-git-server.com/almonds",
						},
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/source/repoURL",
					"oci://127.0.0.1:31999/stefanprodan/charts/podinfo",
				),
				operations.ReplacePatchOperation(
					"/spec/sources/0/repoURL",
					"oci://127.0.0.1:31999/stefanprodan/manifests/podinfo",
				),
				operations.ReplacePatchOperation(
					"/spec/sources/1/repoURL",
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
			name: "should be mutated for OCI repo with internal service registry",
			admissionReq: createArgoAppAdmissionRequest(t, v1.Create, &Application{
				Spec: ApplicationSpec{
					Source: &ApplicationSource{RepoURL: "oci://ghcr.io/stefanprodan/charts/podinfo"},
					Sources: []ApplicationSource{
						{
							RepoURL: "oci://ghcr.io/stefanprodan/manifests/podinfo",
						},
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/source/repoURL",
					"oci://10.11.12.13:5000/stefanprodan/charts/podinfo",
				),
				operations.ReplacePatchOperation(
					"/spec/sources/0/repoURL",
					"oci://10.11.12.13:5000/stefanprodan/manifests/podinfo",
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
			name: "should return internal server error on bad git URL",
			admissionReq: createArgoAppAdmissionRequest(t, v1.Create, &Application{
				Spec: ApplicationSpec{
					Source: &ApplicationSource{RepoURL: "https://bad-url"},
				},
			}),
			code:        http.StatusInternalServerError,
			errContains: AgentErrTransformGitURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.svc != nil {
				_, err := c.Clientset.CoreV1().Services("zarf").Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
