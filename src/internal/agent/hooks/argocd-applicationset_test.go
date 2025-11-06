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

func createArgoAppSetAdmissionRequest(t *testing.T, op v1.Operation, argoAppSet *ApplicationSet) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(argoAppSet)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestArgoAppSetWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{GitServer: state.GitServerInfo{
		Address:      "https://git-server.com",
		PushUsername: "a-push-user",
	}}
	c := createTestClientWithZarfState(ctx, t, s)
	handler := admission.NewHandler().Serve(ctx, NewApplicationSetMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "should mutate git generators and template sources",
			admissionReq: createArgoAppSetAdmissionRequest(t, v1.Create, &ApplicationSet{
				Spec: ApplicationSetSpec{
					Generators: []ApplicationSetGenerator{
						{
							Git: &GitGenerator{
								RepoURL: "https://diff-git-server.com/walnuts",
							},
						},
						{
							Git: &GitGenerator{
								RepoURL: "https://diff-git-server.com/pecans",
							},
						},
					},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/generators/0/git/repoURL",
					"https://git-server.com/a-push-user/walnuts-1104520479",
				),
				operations.ReplacePatchOperation(
					"/spec/generators/1/git/repoURL",
					"https://git-server.com/a-push-user/pecans-1381863636",
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
			admissionReq: createArgoAppSetAdmissionRequest(t, v1.Create, &ApplicationSet{
				Spec: ApplicationSetSpec{
					Generators: []ApplicationSetGenerator{
						{
							Git: &GitGenerator{
								RepoURL: "https://bad-url",
							},
						},
					},
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
