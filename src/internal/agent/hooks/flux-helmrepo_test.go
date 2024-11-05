// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
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

func createFluxHelmRepoAdmissionRequest(t *testing.T, op v1.Operation, fluxHelmRepo *flux.HelmRepository) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(fluxHelmRepo)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestFluxHelmMutationWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := &types.ZarfState{RegistryInfo: types.RegistryInfo{Address: "127.0.0.1:31999"}}

	tests := []admissionTest{
		{
			name: "should not mutate when type is not oci",
			admissionReq: createFluxHelmRepoAdmissionRequest(t, v1.Update, &flux.HelmRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-oci",
				},
				Spec: flux.HelmRepositorySpec{
					Type: "default",
				},
			}),
			code: http.StatusOK,
		},
		{
			name: "error on bad url",
			admissionReq: createFluxHelmRepoAdmissionRequest(t, v1.Update, &flux.HelmRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad-url",
				},
				Spec: flux.HelmRepositorySpec{
					Type: "oci",
					URL:  "bad-url$",
				},
			}),
			errContains: "unable to transform the HelmRepo URL",
			code:        http.StatusInternalServerError,
		},
		{
			name: "should not mutate when agent patched",
			admissionReq: createFluxHelmRepoAdmissionRequest(t, v1.Update, &flux.HelmRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "already-patched",
					Labels: map[string]string{
						"zarf-agent": "patched",
					},
				},
				Spec: flux.HelmRepositorySpec{
					Type: "oci",
				},
			}),
			code: http.StatusOK,
		},
		{
			name: "should be mutated with no internal service registry",
			admissionReq: createFluxHelmRepoAdmissionRequest(t, v1.Create, &flux.HelmRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.HelmRepositorySpec{
					URL:  "oci://ghcr.io/stefanprodan/charts",
					Type: "oci",
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"oci://127.0.0.1:31999/stefanprodan/charts",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfImagePullSecretName},
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
			name: "should be mutated with internal service registry",
			admissionReq: createFluxHelmRepoAdmissionRequest(t, v1.Create, &flux.HelmRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.HelmRepositorySpec{
					URL:  "oci://ghcr.io/stefanprodan/charts",
					Type: "oci",
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"oci://10.11.12.13:5000/stefanprodan/charts",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfImagePullSecretName},
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
							NodePort: int32(31999),
							Port:     5000,
						},
					},
					ClusterIP: "10.11.12.13",
				},
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := createTestClientWithZarfState(ctx, t, state)
			handler := admission.NewHandler().Serve(ctx, NewHelmRepositoryMutationHook(ctx, c))
			if tt.svc != nil {
				_, err := c.Clientset.CoreV1().Services("zarf").Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
