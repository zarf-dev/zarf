// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createFluxOCIRepoAdmissionRequest(t *testing.T, op v1.Operation, fluxOCIRepo *flux.OCIRepository) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(fluxOCIRepo)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestFluxOCIMutationWebhook(t *testing.T) {
	t.Parallel()

	tests := []admissionTest{
		{
			name: "bad oci url",
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Update, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bad oci url",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "bad://ghcr.io/$",
				},
			}),
			errContains: "unable to transform the OCIRepo URL",
			code:        http.StatusInternalServerError,
		},
		{
			name: "should be mutated with no internal service registry",
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Update, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "oci://ghcr.io/stefanprodan/manifests/podinfo",
					Reference: &flux.OCIRepositoryRef{
						Tag: "6.4.0",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"oci://127.0.0.1:31999/stefanprodan/manifests/podinfo",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfImagePullSecretName},
				),
				operations.ReplacePatchOperation(
					"/spec/ref/tag",
					"6.4.0-zarf-2823281104",
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
			name: "test semver tag",
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Update, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "oci://ghcr.io/stefanprodan/manifests/podinfo",
					Reference: &flux.OCIRepositoryRef{
						SemVer: ">= 6.4.0",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"oci://127.0.0.1:31999/stefanprodan/manifests/podinfo",
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
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Create, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "oci://ghcr.io/stefanprodan/charts",
					Reference: &flux.OCIRepositoryRef{
						Digest: "sha256:6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b",
					},
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
		{
			name: "should not mutate URL if it has the same hostname as Zarf s",
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Update, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "oci://127.0.0.1:31999/stefanprodan/manifests/podinfo",
					Reference: &flux.OCIRepositoryRef{
						Tag: "6.4.0-zarf-2823281104",
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"oci://127.0.0.1:31999/stefanprodan/manifests/podinfo",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfImagePullSecretName},
				),
				operations.ReplacePatchOperation(
					"/spec/ref/tag",
					"6.4.0-zarf-2823281104",
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
			name: "should not mutate URL if it has the same hostname as Zarf s internal repo",
			admissionReq: createFluxOCIRepoAdmissionRequest(t, v1.Update, &flux.OCIRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.OCIRepositorySpec{
					URL: "oci://10.11.12.13:5000/stefanprodan/charts",
					Reference: &flux.OCIRepositoryRef{
						Digest: "sha256:6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b",
					},
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

	ctx := context.Background()
	s := &state.State{RegistryInfo: types.RegistryInfo{Address: "127.0.0.1:31999"}}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := createTestClientWithZarfState(ctx, t, s)
			handler := admission.NewHandler().Serve(ctx, NewOCIRepositoryMutationHook(ctx, c))
			if tt.svc != nil {
				_, err := c.Clientset.CoreV1().Services("zarf").Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
