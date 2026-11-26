// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	flux "github.com/fluxcd/source-controller/api/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// guardSubject is a hook wired through withMutationGuard, plus a request that
// the hook would act on if the guard lets it through.
type guardSubject struct {
	name    string
	newHook func(c *cluster.Cluster, mode state.MutationPolicy) operations.Hook
	newReq  func(t *testing.T, labels map[string]string) *v1.AdmissionRequest
	// proceeded is the response expected once the guard delegates to the hook.
	// A 200 code means "a non-empty patch was returned".
	proceeded admissionTest
}

func withLabels(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, base)
	maps.Copy(out, extra)
	return out
}

func guardSubjects() []guardSubject {
	patched := admissionTest{code: http.StatusOK}
	return []guardSubject{
		{
			name:    "pod",
			newHook: NewPodMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createPodAdmissionRequest(t, v1.Create, &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx"}}},
				}, "")
			},
			proceeded: patched,
		},
		{
			name:    "flux GitRepository",
			newHook: NewGitRepositoryMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createFluxGitRepoAdmissionRequest(t, v1.Create, &flux.GitRepository{
					ObjectMeta: metav1.ObjectMeta{Name: "repo", Labels: labels},
					Spec:       flux.GitRepositorySpec{URL: "https://github.com/stefanprodan/podinfo.git"},
				})
			},
			proceeded: patched,
		},
		{
			name:    "flux HelmRepository",
			newHook: NewHelmRepositoryMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createFluxHelmRepoAdmissionRequest(t, v1.Create, &flux.HelmRepository{
					ObjectMeta: metav1.ObjectMeta{Name: "helm", Labels: labels},
					Spec:       flux.HelmRepositorySpec{Type: "oci", URL: "oci://ghcr.io/stefanprodan/charts"},
				})
			},
			proceeded: patched,
		},
		{
			// A successful OCIRepository mutation fetches a manifest from the registry,
			// so an invalid URL is used instead: the hook's own error proves the guard passed.
			name:    "flux OCIRepository",
			newHook: NewOCIRepositoryMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createFluxOCIRepoAdmissionRequest(t, v1.Create, &flux.OCIRepository{
					ObjectMeta: metav1.ObjectMeta{Name: "oci", Labels: labels},
					Spec:       flux.OCIRepositorySpec{URL: "bad://ghcr.io/$"},
				})
			},
			proceeded: admissionTest{code: http.StatusInternalServerError, errContains: AgentErrTransformOCIURL},
		},
		{
			name:    "argocd Application",
			newHook: NewApplicationMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createArgoAppAdmissionRequest(t, v1.Create, &Application{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec:       ApplicationSpec{Source: &ApplicationSource{RepoURL: "https://diff-git-server.com/peanuts"}},
				})
			},
			proceeded: patched,
		},
		{
			name:    "argocd ApplicationSet",
			newHook: NewApplicationSetMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createArgoAppSetAdmissionRequest(t, v1.Create, &ApplicationSet{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: ApplicationSetSpec{Generators: []ApplicationSetGenerator{
						{Git: &GitGenerator{RepoURL: "https://diff-git-server.com/walnuts"}},
					}},
				})
			},
			proceeded: patched,
		},
		{
			name:    "argocd AppProject",
			newHook: NewAppProjectMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createArgoAppProjectAdmissionRequest(t, v1.Create, &AppProject{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec:       AppProjectSpec{SourceRepos: []string{"https://diff-git-server.com/cashews"}},
				})
			},
			proceeded: patched,
		},
		{
			name:    "argocd repository secret",
			newHook: NewRepositorySecretMutationHook,
			newReq: func(t *testing.T, labels map[string]string) *v1.AdmissionRequest {
				return createArgoRepoAdmissionRequest(t, v1.Create, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "argo-repo-secret",
						Labels: withLabels(map[string]string{"argocd.argoproj.io/secret-type": "repository"}, labels),
					},
					Data: map[string][]byte{"url": []byte("https://diff-git-server.com/podinfo")},
				})
			},
			proceeded: patched,
		},
	}
}

func TestWithMutationGuard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	s := &state.State{
		GitServer: state.GitServerInfo{
			Address:      "https://git-server.com",
			PushUsername: "a-push-user",
			PullUsername: "a-pull-user",
			PullPassword: "a-pull-password",
		},
		RegistryInfo: state.RegistryInfo{
			Address:      "127.0.0.1:31999",
			Port:         31999,
			PullUsername: "registry-pull-user",
			PullPassword: "registry-pull-password",
		},
	}
	c := createTestClientWithZarfState(ctx, t, s)

	const (
		ignoredNamespace = "guard-ignored"
		mutateNamespace  = "guard-mutate"
		missingNamespace = "guard-missing"
	)
	for name, value := range map[string]string{ignoredNamespace: "ignore", mutateNamespace: "mutate"} {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{cluster.AgentLabel: value}}}
		_, err := c.Clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	type outcome int
	const (
		proceed outcome = iota
		skip
		namespaceErr
	)

	cases := []struct {
		name          string
		mode          state.MutationPolicy
		resourceLabel string
		namespace     string
		want          outcome
	}{
		{name: "all/unlabeled", mode: state.MutationPolicyAll, namespace: testNamespace, want: proceed},
		{name: "all/resource ignore", mode: state.MutationPolicyAll, resourceLabel: "ignore", namespace: testNamespace, want: skip},
		{name: "all/resource skip", mode: state.MutationPolicyAll, resourceLabel: "skip", namespace: testNamespace, want: skip},
		{name: "all/namespace ignore", mode: state.MutationPolicyAll, namespace: ignoredNamespace, want: skip},
		{name: "all/resource mutate overrides namespace ignore", mode: state.MutationPolicyAll, resourceLabel: "mutate", namespace: ignoredNamespace, want: proceed},
		{name: "all/namespace not found", mode: state.MutationPolicyAll, namespace: missingNamespace, want: namespaceErr},
		{name: "labeled/unlabeled", mode: state.MutationPolicyLabeled, namespace: testNamespace, want: skip},
		{name: "labeled/namespace mutate", mode: state.MutationPolicyLabeled, namespace: mutateNamespace, want: proceed},
		{name: "labeled/resource mutate", mode: state.MutationPolicyLabeled, resourceLabel: "mutate", namespace: testNamespace, want: proceed},
		{name: "labeled/resource ignore overrides namespace mutate", mode: state.MutationPolicyLabeled, resourceLabel: "ignore", namespace: mutateNamespace, want: skip},
		{name: "labeled/namespace not found", mode: state.MutationPolicyLabeled, namespace: missingNamespace, want: namespaceErr},
	}

	for _, subject := range guardSubjects() {
		for _, tc := range cases {
			t.Run(subject.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				var labels map[string]string
				if tc.resourceLabel != "" {
					labels = map[string]string{cluster.AgentLabel: tc.resourceLabel}
				}
				req := subject.newReq(t, labels)
				req.Namespace = tc.namespace

				handler := admission.NewHandler().Serve(ctx, subject.newHook(c, tc.mode))
				rr := sendAdmissionRequest(t, req, handler)

				var review v1.AdmissionReview
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &review))

				switch tc.want {
				case skip:
					require.True(t, review.Response.Allowed)
					verifyAdmission(t, rr, admissionTest{code: http.StatusOK})
				case namespaceErr:
					verifyAdmission(t, rr, admissionTest{
						code:        http.StatusInternalServerError,
						errContains: "failed to get namespace " + missingNamespace,
					})
				case proceed:
					if subject.proceeded.code == http.StatusOK {
						require.Equal(t, http.StatusOK, rr.Code)
						require.True(t, review.Response.Allowed)
						require.NotEmpty(t, review.Response.Patch, "expected the hook to mutate once the guard let the request through")
						return
					}
					verifyAdmission(t, rr, subject.proceeded)
				}
			})
		}
	}
}
