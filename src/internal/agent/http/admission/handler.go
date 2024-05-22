// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package admission provides an HTTP handler for a Kubernetes admission webhook.
// It includes functionality to decode incoming admission requests, execute
// the corresponding operations, and return appropriate admission responses.
package admission

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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
func (h *Handler) Serve(hook operations.Hook) http.HandlerFunc {
	message.Debugf("http.Serve(%#v)", hook)
	return func(w http.ResponseWriter, r *http.Request) {
		message.Debugf("http.Serve()(writer, %#v)", r.URL)

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
		if err != nil {
			message.Warnf("%s: %s", lang.AgentErrBindHandler, err.Error())
			admissionResponse := corev1.AdmissionReview{
				Response: &corev1.AdmissionResponse{
					Result: &metav1.Status{Message: err.Error(), Status: string(metav1.StatusReasonInternalError)},
				},
			}
			jsonResponse, err := json.Marshal(admissionResponse)
			if err != nil {
				message.WarnErr(err, lang.AgentErrMarshalResponse)
				http.Error(w, lang.AgentErrMarshalResponse, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(jsonResponse)
			return
		}

		admissionResponse := corev1.AdmissionReview{
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
				message.WarnErr(err, lang.AgentErrMarshallJSONPatch)
				http.Error(w, lang.AgentErrMarshallJSONPatch, http.StatusInternalServerError)
			}
			admissionResponse.Response.Patch = patchBytes
			admissionResponse.Response.PatchType = &jsonPatchType
		}

		jsonResponse, err := json.Marshal(admissionResponse)
		if err != nil {
			message.WarnErr(err, lang.AgentErrMarshalResponse)
			http.Error(w, lang.AgentErrMarshalResponse, http.StatusInternalServerError)
			return
		}

		message.Debug("PATCH: ", string(admissionResponse.Response.Patch))
		message.Debug("RESPONSE: ", string(jsonResponse))

		message.Infof(lang.AgentInfoWebhookAllowed, r.URL.Path, review.Request.Operation, result.Allowed)
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}
