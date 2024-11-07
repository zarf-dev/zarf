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
	"github.com/zarf-dev/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
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
	state := &types.ZarfState{GitServer: types.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
	}}
	c := createTestClientWithZarfState(ctx, t, state)
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
			name: "should mutate even if agent patched",
			admissionReq: createArgoAppAdmissionRequest(t, v1.Create, &Application{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"zarf-agent": "patched",
					},
				},
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
