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
	"github.com/zarf-dev/zarf/src/types"
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
	s := &state.State{GitServer: types.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
		PullPassword: "a-pull-password",
		PullUsername: "a-pull-user",
	}}
	c := createTestClientWithZarfState(ctx, t, s)
	handler := admission.NewHandler().Serve(ctx, NewRepositorySecretMutationHook(ctx, c))

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
