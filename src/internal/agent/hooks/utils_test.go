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

	"github.com/defenseunicorns/zarf/src/internal/agent/http/admission"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// setupWebhookTest sets up the test environment and returns the http.HandlerFunc for the provided hook.
// Cluster.K8s.Clientset should be initialized with fake.NewSimpleClientset() prior to passing into this function.
func setupWebhookTest(ctx context.Context, t *testing.T, c *cluster.Cluster, state *types.ZarfState, hookFunc func(ctx context.Context, c *cluster.Cluster) operations.Hook) http.HandlerFunc {
	t.Helper()
	createTestZarfStateSecret(ctx, t, c, state)
	return admission.NewHandler().Serve(hookFunc(ctx, c))
}

// createTestZarfStateSecret creates a test zarf-state secret in the zarf namespace.
// Cluster.K8s.Clientset should be initialized with fake.NewSimpleClientset() prior to passing into this function.
func createTestZarfStateSecret(ctx context.Context, t *testing.T, c *cluster.Cluster, state *types.ZarfState) {
	t.Helper()
	stateData, err := json.Marshal(state)
	require.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.ZarfStateSecretName,
			Namespace: cluster.ZarfNamespaceName,
		},
		Data: map[string][]byte{
			cluster.ZarfStateDataKey: stateData,
		},
	}
	_, err = c.Clientset.CoreV1().Secrets(cluster.ZarfNamespaceName).Create(ctx, secret, metav1.CreateOptions{})
	require.NoError(t, err)
}

// sendAdmissionRequest sends an admission request to the handler and returns the response.
func sendAdmissionRequest(t *testing.T, admissionReq *v1.AdmissionRequest, handler http.HandlerFunc, code int) *v1.AdmissionResponse {
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

	require.Equal(t, code, rr.Code)

	var admissionReview v1.AdmissionReview
	if rr.Code == http.StatusOK {
		err = json.NewDecoder(rr.Body).Decode(&admissionReview)
		require.NoError(t, err)
	}

	return admissionReview.Response
}
