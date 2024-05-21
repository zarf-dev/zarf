// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
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
	c := &cluster.Cluster{K8s: &k8s.K8s{Clientset: fake.NewSimpleClientset()}}
	state := &types.ZarfState{GitServer: types.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
		PullUsername: "a-pull-password",
	}}
	handler := setupWebhookTest(ctx, t, c, state, NewRepositorySecretMutationHook)

	tests := []struct {
		name          string
		admissionReq  *v1.AdmissionRequest
		expectedPatch []operations.PatchOperation
		code          int
	}{
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
			expectedPatch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/data/url",
					b64.StdEncoding.EncodeToString([]byte("https://git-server.com/a-push-user/podinfo-1868163476")),
				),
				operations.ReplacePatchOperation(
					"/data/username",
					//TODO this should be different
					b64.StdEncoding.EncodeToString([]byte("zarf-git-read-user")),
				),
				operations.ReplacePatchOperation(
					"/data/password",
					//TODO this should be different
					b64.StdEncoding.EncodeToString([]byte(state.GitServer.PullPassword)),
				),
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := sendAdmissionRequest(t, tt.admissionReq, handler, tt.code)
			if tt.expectedPatch != nil {
				expectedPatchJSON, err := json.Marshal(tt.expectedPatch)
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.True(t, resp.Allowed)
				require.JSONEq(t, string(expectedPatchJSON), string(resp.Patch))
			} else if tt.code != http.StatusInternalServerError {
				require.Empty(t, string(resp.Patch))
			}
		})
	}
}
