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
	s := &state.State{GitServer: state.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
	}}
	c := createTestClientWithZarfState(ctx, t, s)
	handler := admission.NewHandler().Serve(ctx, NewAppProjectMutationHook(ctx, c))

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
			t.Parallel()
			rr := sendAdmissionRequest(t, tt.admissionReq, handler)
			verifyAdmission(t, rr, tt)
		})
	}
}
