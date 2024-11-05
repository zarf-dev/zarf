// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package admission provides an HTTP handler for a Kubernetes admission webhook.
// It includes functionality to decode incoming admission requests, execute
// the corresponding operations, and return appropriate admission responses.
package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	corev1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// Handler represents the HTTP handler for an admission webhook.
type Handler struct {
	decoder runtime.Decoder
}

// NewHandler returns a new admission Handler.
func NewHandler() *Handler {
	return &Handler{
		decoder: serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer(),
	}
}

// Serve returns an http.HandlerFunc for an admission webhook.
func (h *Handler) Serve(ctx context.Context, hook operations.Hook) http.HandlerFunc {
	l := logger.From(ctx)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, lang.AgentErrInvalidMethod, http.StatusMethodNotAllowed)
			return
		}

		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			http.Error(w, lang.AgentErrInvalidType, http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf(lang.AgentErrBadRequest, err), http.StatusBadRequest)
			return
		}

		var review corev1.AdmissionReview
		if _, _, err := h.decoder.Decode(body, nil, &review); err != nil {
			http.Error(w, fmt.Sprintf(lang.AgentErrCouldNotDeserializeReq, err), http.StatusBadRequest)
			return
		}

		if review.Request == nil {
			http.Error(w, lang.AgentErrNilReq, http.StatusBadRequest)
			return
		}

		result, err := hook.Execute(review.Request)
		admissionMeta := metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "AdmissionReview",
		}
		if err != nil {
			l.Error("unable to bind the webhook handler", "error", err.Error())
			admissionResponse := corev1.AdmissionReview{
				TypeMeta: admissionMeta,
				Response: &corev1.AdmissionResponse{
					Result: &metav1.Status{Message: err.Error(), Status: string(metav1.StatusReasonInternalError)},
				},
			}
			jsonResponse, err := json.Marshal(admissionResponse)
			if err != nil {
				l.Error("unable to marshal the response", "error", err.Error())
				http.Error(w, lang.AgentErrMarshalResponse, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			//nolint:errcheck // ignore
			w.Write(jsonResponse)
			return
		}

		admissionResponse := corev1.AdmissionReview{
			TypeMeta: admissionMeta,
			Response: &corev1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: result.Allowed,
				Result:  &metav1.Status{Message: result.Msg},
			},
		}

		// Set the patch operations for mutating admission
		if len(result.PatchOps) > 0 {
			jsonPatchType := corev1.PatchTypeJSONPatch
			patchBytes, err := json.Marshal(result.PatchOps)
			if err != nil {
				l.Error("unable to marshall the json patch", "error", err.Error())
				http.Error(w, lang.AgentErrMarshallJSONPatch, http.StatusInternalServerError)
			}
			admissionResponse.Response.Patch = patchBytes
			admissionResponse.Response.PatchType = &jsonPatchType
		}

		jsonResponse, err := json.Marshal(admissionResponse)
		if err != nil {
			l.Error("unable to marshal the response", "error", err)
			http.Error(w, lang.AgentErrMarshalResponse, http.StatusInternalServerError)
			return
		}

		message.Infof(lang.AgentInfoWebhookAllowed, r.URL.Path, review.Request.Operation, result.Allowed)
		l.Info("webhook execution complete", "path", r.URL.Path, "operation", review.Request.Operation, "allowed", result.Allowed)
		w.WriteHeader(http.StatusOK)
		//nolint: errcheck // ignore
		w.Write(jsonResponse)
	}
}
