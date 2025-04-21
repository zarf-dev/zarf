// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type admissionTest struct {
	name         string
	admissionReq *v1.AdmissionRequest
	patch        []operations.PatchOperation
	code         int
	errContains  string
	svc          *corev1.Service
}

func createTestClientWithZarfState(ctx context.Context, t *testing.T, s *state.State) *cluster.Cluster {
	t.Helper()
	c := &cluster.Cluster{Clientset: fake.NewClientset()}
	stateData, err := json.Marshal(s)
	require.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      state.ZarfStateSecretName,
			Namespace: state.ZarfNamespaceName,
		},
		Data: map[string][]byte{
			state.ZarfStateDataKey: stateData,
		},
	}
	_, err = c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).Create(ctx, secret, metav1.CreateOptions{})
	require.NoError(t, err)
	return c
}

// sendAdmissionRequest sends an admission request to the handler and returns the response.
func sendAdmissionRequest(t *testing.T, admissionReq *v1.AdmissionRequest, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	b, err := json.Marshal(&v1.AdmissionReview{
		Request: admissionReq,
	})
	require.NoError(t, err)

	// Note: The URL ("/test") doesn't matter here because we are directly invoking the handler.
	// The handler processes the request based on the HTTP method and body content, not the URL path.
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return rr
}

func verifyAdmission(t *testing.T, rr *httptest.ResponseRecorder, expected admissionTest) {
	t.Helper()

	require.Equal(t, expected.code, rr.Code)

	var admissionReview v1.AdmissionReview

	err := json.NewDecoder(rr.Body).Decode(&admissionReview)

	if expected.errContains != "" {
		require.Contains(t, admissionReview.Response.Result.Message, expected.errContains)
		return
	}

	resp := admissionReview.Response
	require.NoError(t, err)
	if expected.patch == nil {
		require.Empty(t, string(resp.Patch))
	} else {
		expectedPatchJSON, err := json.Marshal(expected.patch)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Allowed)
		require.JSONEq(t, string(expectedPatchJSON), string(resp.Patch))
	}
}
