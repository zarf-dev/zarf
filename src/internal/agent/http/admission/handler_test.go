// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// headerCountingRecorder counts WriteHeader calls, since httptest.ResponseRecorder
// silently ignores every call after the first.
type headerCountingRecorder struct {
	*httptest.ResponseRecorder
	headerWrites int
}

func (r *headerCountingRecorder) WriteHeader(code int) {
	r.headerWrites++
	r.ResponseRecorder.WriteHeader(code)
}

func (r *headerCountingRecorder) Write(b []byte) (int, error) {
	if r.headerWrites == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseRecorder.Write(b)
}

func hookReturning(result *operations.Result, err error) operations.Hook {
	return operations.Hook{
		Create: func(_ context.Context, _ *v1.AdmissionRequest) (*operations.Result, error) {
			return result, err
		},
	}
}

func reviewBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(&v1.AdmissionReview{
		Request: &v1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Operation: v1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{}`)},
		},
	})
	require.NoError(t, err)
	return b
}

func TestHandlerServe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		method      string
		contentType string
		body        io.Reader
		hook        operations.Hook
		wantCode    int
		// wantBody is the exact plain-text body written by http.Error.
		wantBody string
		// wantBodyPrefix is used when the body ends with a decoder-specific error.
		wantBodyPrefix string
		// wantReviewMsg is the message expected inside a JSON AdmissionReview response.
		wantReviewMsg string
		wantPatch     bool
	}{
		{
			name:     "wrong method",
			method:   http.MethodGet,
			body:     http.NoBody,
			wantCode: http.StatusMethodNotAllowed,
			wantBody: lang.AgentErrInvalidMethod + "\n",
		},
		{
			name:        "wrong content type",
			contentType: "text/plain",
			body:        bytes.NewReader([]byte(`{}`)),
			wantCode:    http.StatusBadRequest,
			wantBody:    lang.AgentErrInvalidType + "\n",
		},
		{
			name:     "unreadable body",
			body:     iotest.ErrReader(errors.New("read failed")),
			wantCode: http.StatusBadRequest,
			wantBody: "could not read request body: read failed\n",
		},
		{
			name:           "undecodable body",
			body:           bytes.NewReader([]byte(`not json`)),
			wantCode:       http.StatusBadRequest,
			wantBodyPrefix: "could not deserialize request: ",
		},
		{
			name:     "nil request",
			body:     bytes.NewReader([]byte(`{}`)),
			wantCode: http.StatusBadRequest,
			wantBody: lang.AgentErrNilReq + "\n",
		},
		{
			name:          "hook error",
			hook:          hookReturning(nil, errors.New("hook failed")),
			wantCode:      http.StatusInternalServerError,
			wantReviewMsg: "hook failed",
		},
		{
			name: "unmarshalable patch",
			hook: hookReturning(&operations.Result{
				Allowed:  true,
				PatchOps: []operations.PatchOperation{{Op: "add", Path: "/x", Value: make(chan int)}},
			}, nil),
			wantCode: http.StatusInternalServerError,
			wantBody: lang.AgentErrMarshallJSONPatch + "\n",
		},
		{
			name: "patch returned",
			hook: hookReturning(&operations.Result{
				Allowed:  true,
				PatchOps: []operations.PatchOperation{operations.AddPatchOperation("/x", "y")},
			}, nil),
			wantCode:  http.StatusOK,
			wantPatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			method := tt.method
			if method == "" {
				method = http.MethodPost
			}
			contentType := tt.contentType
			if contentType == "" {
				contentType = "application/json"
			}
			body := tt.body
			if body == nil {
				body = bytes.NewReader(reviewBody(t))
			}

			req := httptest.NewRequest(method, "/mutate/test", body)
			req.Header.Set("Content-Type", contentType)
			rr := &headerCountingRecorder{ResponseRecorder: httptest.NewRecorder()}

			NewHandler().Serve(context.Background(), tt.hook).ServeHTTP(rr, req)

			require.Equal(t, tt.wantCode, rr.Code)
			require.Equal(t, 1, rr.headerWrites, "expected exactly one response to be written")

			switch {
			case tt.wantBody != "":
				require.Equal(t, tt.wantBody, rr.Body.String())
			case tt.wantBodyPrefix != "":
				require.True(t, strings.HasPrefix(rr.Body.String(), tt.wantBodyPrefix), "body: %q", rr.Body.String())
			case tt.wantReviewMsg != "" || tt.wantPatch:
				var review v1.AdmissionReview
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &review))
				require.NotNil(t, review.Response)
				if tt.wantReviewMsg != "" {
					require.Equal(t, tt.wantReviewMsg, review.Response.Result.Message)
				}
				if tt.wantPatch {
					require.Equal(t, types.UID("test-uid"), review.Response.UID)
					require.True(t, review.Response.Allowed)
					require.JSONEq(t, `[{"op":"add","path":"/x","value":"y"}]`, string(review.Response.Patch))
				}
			}
		})
	}
}
