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

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func createTestClientWithZarfState(ctx context.Context, t *testing.T, state *types.ZarfState) *cluster.Cluster {
	t.Helper()
	c := &cluster.Cluster{K8s: &k8s.K8s{Clientset: fake.NewSimpleClientset()}}
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
	return c
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
