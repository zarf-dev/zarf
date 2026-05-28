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
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// When state has no git server address (cluster was initialized without gitea
// and without an external --git-url), the agent must not mutate any git resource.
func TestGitMutationHooksSkipWhenUnConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := &state.State{} // empty GitServer → IsConfigured() == false
	c := createTestClientWithZarfState(ctx, t, s)

	mustRaw := func(v any) runtime.RawExtension {
		t.Helper()
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return runtime.RawExtension{Raw: b}
	}

	cases := []admissionTest{
		{
			name: "argocd Application",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Object: mustRaw(&Application{
					Spec: ApplicationSpec{Source: &ApplicationSource{RepoURL: "https://example.com/org/repo"}},
				}),
			},
			code: http.StatusOK,
		},
		{
			name: "argocd ApplicationSet",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Object: mustRaw(&ApplicationSet{
					Spec: ApplicationSetSpec{Generators: []ApplicationSetGenerator{
						{Git: &GitGenerator{RepoURL: "https://example.com/org/repo"}},
					}},
				}),
			},
			code: http.StatusOK,
		},
		{
			name: "argocd AppProject",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Object: mustRaw(&AppProject{
					Spec: AppProjectSpec{SourceRepos: []string{"https://example.com/org/repo"}},
				}),
			},
			code: http.StatusOK,
		},
		{
			name: "argocd repository secret",
			admissionReq: &v1.AdmissionRequest{
				Operation: v1.Create,
				Object: mustRaw(&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "repo-creds"},
					Data:       map[string][]byte{"url": []byte("https://example.com/org/repo")},
				}),
			},
			code: http.StatusOK,
		},
	}

	hooks := map[string]http.HandlerFunc{
		"argocd Application":       admission.NewHandler().Serve(ctx, NewApplicationMutationHook(ctx, c)),
		"argocd ApplicationSet":    admission.NewHandler().Serve(ctx, NewApplicationSetMutationHook(ctx, c)),
		"argocd AppProject":        admission.NewHandler().Serve(ctx, NewAppProjectMutationHook(ctx, c)),
		"argocd repository secret": admission.NewHandler().Serve(ctx, NewRepositorySecretMutationHook(ctx, c)),
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := sendAdmissionRequest(t, tc.admissionReq, hooks[tc.name])
			verifyAdmission(t, rr, tc)
		})
	}

	// Flux GitRepository uses a separate unstructured type; exercise it inline.
	t.Run("flux GitRepository", func(t *testing.T) {
		t.Parallel()
		raw := runtime.RawExtension{Raw: []byte(`{"spec":{"url":"https://example.com/org/repo"}}`)}
		req := &v1.AdmissionRequest{Operation: v1.Create, Object: raw}
		handler := admission.NewHandler().Serve(ctx, NewGitRepositoryMutationHook(ctx, c))
		rr := sendAdmissionRequest(t, req, handler)
		verifyAdmission(t, rr, admissionTest{code: http.StatusOK})
	})
}
