// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
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

	tests := []admissionTest{
		{
			name: "should be mutated",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Create, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.GitRepositorySpec{
					URL: "https://github.com/stefanprodan/podinfo.git",
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"https://git-server.com/a-push-user/podinfo-1646971829.git",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfGitServerSecretName},
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
			name: "should not mutate invalid git url",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Update, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mutate-this",
				},
				Spec: flux.GitRepositorySpec{
					URL: "not-a-git-url",
				},
			}),
			patch:       nil,
			code:        http.StatusInternalServerError,
			errContains: AgentErrTransformGitURL,
		},
		{
			name: "should patch to same url and update secret if hostname matches",
			admissionReq: createFluxGitRepoAdmissionRequest(t, v1.Update, &flux.GitRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-mutate",
				},
				Spec: flux.GitRepositorySpec{
					URL: "https://git-server.com/a-push-user/podinfo.git",
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/url",
					"https://git-server.com/a-push-user/podinfo.git",
				),
				operations.AddPatchOperation(
					"/spec/secretRef",
					fluxmeta.LocalObjectReference{Name: config.ZarfGitServerSecretName},
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
