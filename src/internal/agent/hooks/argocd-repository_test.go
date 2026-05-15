// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	b64 "encoding/base64"
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

func createArgoRepoAdmissionRequest(t *testing.T, op v1.Operation, argoRepo *corev1.Secret) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(argoRepo)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestArgoRepoWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{
		GitServer: state.GitServerInfo{
			Address:      "https://git-server.com",
			PushUsername: "a-push-user",
			PullPassword: "a-pull-password",
			PullUsername: "a-pull-user",
		},
		RegistryInfo: state.RegistryInfo{
			Address:      "127.0.0.1:31999",
			NodePort:     31999,
			PullUsername: "registry-pull-user",
			PullPassword: "registry-pull-password",
		},
	}

	tests := []admissionTest{
		{
			name: "should be mutated",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url": []byte("https://diff-git-server.com/podinfo"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("https://git-server.com/a-push-user/podinfo-1868163476")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI repo",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-oci-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://registry-1.docker.io/dhpup/oci-edge"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://127.0.0.1:31999/dhpup/oci-edge")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI Helm repo",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-oci-helm-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":       []byte("oci://ghcr.io/stefanprodan/charts/podinfo"),
					"enableOCI": []byte("true"),
					"type":      []byte("helm"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://127.0.0.1:31999/stefanprodan/charts/podinfo")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI Helm repo with internal service registry",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-oci-helm-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":       []byte("oci://ghcr.io/stefanprodan/charts/podinfo"),
					"enableOCI": []byte("true"),
					"type":      []byte("helm"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts/podinfo")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
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
			name: "should be mutated for repo-creds",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
					},
					Name:      "argo-repo-creds-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url": []byte("https://diff-git-server.com/podinfo"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("https://git-server.com/a-push-user/podinfo-1868163476")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI repo for repo-creds",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
					},
					Name:      "argo-oci-repo-creds-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://registry-1.docker.io/dhpup/oci-edge"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://127.0.0.1:31999/dhpup/oci-edge")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI Helm repo for repo-creds",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
					},
					Name:      "argo-oci-helm-repo-creds-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":       []byte("oci://ghcr.io/stefanprodan/charts/podinfo"),
					"enableOCI": []byte("true"),
					"type":      []byte("helm"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://127.0.0.1:31999/stefanprodan/charts/podinfo")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should be mutated for OCI Helm repo with internal service registry for repo-creds",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
					},
					Name:      "argo-oci-helm-repo-creds-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":       []byte("oci://ghcr.io/stefanprodan/charts/podinfo"),
					"enableOCI": []byte("true"),
					"type":      []byte("helm"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts/podinfo")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repo-creds",
						"zarf-agent":                     "patched",
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
			name: "matching hostname on update should stay the same, but secret should be added",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Update, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url": []byte("https://git-server.com/podinfo"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("https://git-server.com/podinfo")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.GitServer.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should not mutate already patched cluster DNS url",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Update, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
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
			name: "should mutate cluster IP to DNS",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Update, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://10.11.12.13:5000/stefanprodan/charts"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
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
			name: "should be mutated with mTLS enabled",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-oci-repo-mtls-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://ghcr.io/stefanprodan/charts"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://zarf-docker-registry.zarf.svc.cluster.local:5000/stefanprodan/charts")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/tlsClientCertData",
					b64.StdEncoding.EncodeToString([]byte("zarf-registry-client-tls-crt")),
				),
				operations.ReplacePatchOperation(
					"/data/tlsClientCertKey",
					b64.StdEncoding.EncodeToString([]byte("zarf-registry-client-tls-key")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
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
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Port: 5000,
						},
					},
				},
			},
			registryInfo: state.RegistryInfo{
				Address:      "127.0.0.1:31999",
				RegistryMode: state.RegistryModeProxy,
				PullUsername: "registry-pull-user",
				PullPassword: "registry-pull-password",
			},
			useMTLS: true,
			code:    http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testState := s
			if tt.registryInfo.Address != "" {
				testState = &state.State{
					GitServer:    s.GitServer,
					RegistryInfo: tt.registryInfo,
				}
			}
			c := createTestClientWithZarfState(ctx, t, testState)
			handler := admission.NewHandler().Serve(ctx, NewRepositorySecretMutationHook(ctx, c))
			if tt.svc != nil {
				_, err := c.Clientset.CoreV1().Services("zarf").Create(ctx, tt.svc, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			if tt.useMTLS {
				certSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "zarf-registry-client-tls",
						Namespace: state.ZarfNamespaceName,
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("zarf-registry-client-tls-crt"),
						"tls.key": []byte("zarf-registry-client-tls-key"),
						"ca.crt":  []byte("zarf-registry-client-tls-ca-crt"),
					},
				}
				_, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, certSecret, metav1.CreateOptions{})
				require.NoError(t, err)
				testState.RegistryInfo.MTLSStrategy = state.MTLSStrategyZarfManaged
				err = c.SaveState(ctx, testState)
				require.NoError(t, err)
			}
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}

// TestArgoRepoWebhookRegistryOnly verifies behaviour when only the Zarf registry is configured
// and no git server is present (OCI-only deployment).
func TestArgoRepoWebhookRegistryOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{
		// GitServer intentionally not configured (OCI-only scenario).
		RegistryInfo: state.RegistryInfo{
			Address:      "127.0.0.1:31999",
			NodePort:     31999,
			PullUsername: "registry-pull-user",
			PullPassword: "registry-pull-password",
		},
	}

	tests := []admissionTest{
		{
			name: "OCI repository should be mutated when only registry is configured",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-oci-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url":  []byte("oci://registry-1.docker.io/dhpup/oci-edge"),
					"type": []byte("oci"),
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("oci://127.0.0.1:31999/dhpup/oci-edge")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullUsername)),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					b64.StdEncoding.EncodeToString([]byte(s.RegistryInfo.PullPassword)),
				),
				operations.ReplacePatchOperation(
					"/data/insecureOCIForceHttp",
					b64.StdEncoding.EncodeToString([]byte("true")),
				),
				operations.ReplacePatchOperation(
					"/metadata/labels",
					map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
						"zarf-agent":                     "patched",
					},
				),
			},
			code: http.StatusOK,
		},
		{
			// A git repository secret must not be mutated when there is no git server configured,
			// regardless of whether a registry is present.
			name: "git repository should be skipped when only registry is configured",
			admissionReq: createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "repository",
					},
					Name:      "argo-git-repo-secret",
					Namespace: "argo",
				},
				Data: map[string][]byte{
					"url": []byte("https://some-git-server.com/podinfo"),
				},
			}),
			patch: nil,
			code:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := createTestClientWithZarfState(ctx, t, s)
			handler := admission.NewHandler().Serve(ctx, NewRepositorySecretMutationHook(ctx, c))
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}

// TestArgoRepoWebhookGitOnly verifies that OCI repository secrets are not mutated when there is
// no registry configured (git-only deployment).
func TestArgoRepoWebhookGitOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{
		GitServer: state.GitServerInfo{
			Address:      "https://git-server.com",
			PushUsername: "a-push-user",
			PullPassword: "a-pull-password",
			PullUsername: "a-pull-user",
		},
		// RegistryInfo intentionally not configured (git-only scenario).
	}

	admissionReq := createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "repository",
			},
			Name:      "argo-oci-repo-secret",
			Namespace: "argo",
		},
		Data: map[string][]byte{
			"url":  []byte("oci://registry-1.docker.io/dhpup/oci-edge"),
			"type": []byte("oci"),
		},
	})

	c := createTestClientWithZarfState(ctx, t, s)
	handler := admission.NewHandler().Serve(ctx, NewRepositorySecretMutationHook(ctx, c))
	rr := sendAdmissionRequest(t, admissionReq, handler)
	verifyAdmission(t, rr, admissionTest{patch: nil, code: http.StatusOK})
}
