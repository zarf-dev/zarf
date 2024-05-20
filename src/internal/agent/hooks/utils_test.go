// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
)

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
	err = json.NewDecoder(rr.Body).Decode(&admissionReview)
	require.NoError(t, err)

	resp := admissionReview.Response
	require.NotNil(t, resp)
	require.True(t, resp.Allowed)

	return resp
}
