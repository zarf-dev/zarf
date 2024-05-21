// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/http/admission"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/types"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createFluxGitRepoAdmissionRequest(t *testing.T, op v1.Operation, fluxGitRepo *flux.GitRepository) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(fluxGitRepo)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestFluxMutationWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	state := &types.ZarfState{GitServer: types.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
	}}
	c := createTestClientWithZarfState(ctx, t, state)
	handler := admission.NewHandler().Serve(NewGitRepositoryMutationHook(ctx, c))

	tests := []struct {
		name          string
		admissionReq  *v1.AdmissionRequest
		expectedPatch []operations.PatchOperation
		code          int
		err           error
	}{
		{
			name: "should be mutated",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Create, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.GitRepositorySpec{
					URL: "https://github.com/stefanprodan/podinfo.git",
					Reference: &flux.GitRepositoryRef{
						Tag: "6.4.0",
					},
				},
			}),
			expectedPatch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"https://git-server.com/a-push-user/podinfo-1646971829.git",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfGitServerSecretName},
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should not mutate invalid git url",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Update, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.GitRepositorySpec{
					URL: "not-a-git-url",
					Reference: &flux.GitRepositoryRef{
						Tag: "6.4.0",
					},
				},
			}),
			expectedPatch: nil,
			code:          http.StatusInternalServerError,
			err:           fmt.Errorf("unable to transform the git url:"),
		},
		{
			name: "should replace existing secret",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Create, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "replace-secret",
				},
				Spec: flux.GitRepositorySpec{
					URL: "https://github.com/stefanprodan/podinfo.git",
					SecretRef: &fluxmeta.LocalObjectReference{
						Name: "existing-secret",
					},
					Reference: &flux.GitRepositoryRef{
						Tag: "6.4.0",
					},
				},
			}),
			expectedPatch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"https://git-server.com/a-push-user/podinfo-1646971829.git",
				),
				operations.ReplacePatchOperation(
					"/spec/secretRef/name",
					config.ZarfGitServerSecretName,
				),
			},
			code: http.StatusOK,
		},
		{
			name: "should patch to same url and update secret if hostname matches",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Update, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-mutate",
				},
				Spec: flux.GitRepositorySpec{
					URL: "https://git-server.com/a-push-user/podinfo.git",
					Reference: &flux.GitRepositoryRef{
						Tag: "6.4.0",
					},
				},
			}),
			expectedPatch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"https://git-server.com/a-push-user/podinfo.git",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfGitServerSecretName},
				),
			},
			code: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler, tt.code)
			verifyAdmission(t, rr, tt.code, tt.expectedPatch, tt.err)
			// if tt.err != nil {
			// 	resp.Body
			// } else if tt.expectedPatch == nil {
			// 	require.Empty(t, string(resp.Patch))
			// } else {
			// 	expectedPatchJSON, err := json.Marshal(tt.expectedPatch)
			// 	require.NoError(t, err)
			// 	require.NotNil(t, resp)
			// 	require.True(t, resp.Allowed)
			// 	require.JSONEq(t, string(expectedPatchJSON), string(resp.Patch))
			// }
		})
	}
}
