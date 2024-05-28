// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package hooks

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/http/admission"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func createConfigurationAdmissionRequest(t *testing.T, op v1.Operation, configuration *Configuration) *v1.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(configuration)
	require.NoError(t, err)
	return &v1.AdmissionRequest{
		Operation: op,
		Object: runtime.RawExtension{
			Raw: raw,
		},
	}
}

func TestConfigurationMutationWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	state := &types.ZarfState{RegistryInfo: types.RegistryInfo{Address: "127.0.0.1:31999"}}
	c := createTestClientWithZarfState(ctx, t, state)
	handler := admission.NewHandler().Serve(NewConfigurationMutationHook(ctx, c))

	tests := []admissionTest{
		{
			name: "configuration with label should be mutated",
			admissionReq: createConfigurationAdmissionRequest(t, v1.Create, &Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"should-be": "mutated"},
				},
				Spec: ConfigurationSpec{
					PackageSpec: PackageSpec{Package: "busybox"},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/packagePullSecrets",
					[]corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}},
				),
				operations.ReplacePatchOperation(
					"/spec/package",
					"127.0.0.1:31999/library/busybox:latest-zarf-2140033595",
				),
				operations.ReplacePatchOperation(
					"/metadata/labels/zarf-agent",
					"patched",
				),
			},
			code: http.StatusOK,
		},
		{
			name: "configuration with zarf-agent patched label should not be mutated",
			admissionReq: createConfigurationAdmissionRequest(t, v1.Create, &Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"zarf-agent": "patched"},
				},
				Spec: ConfigurationSpec{
					PackageSpec: PackageSpec{Package: "nginx"},
				},
			}),
			patch: nil,
			code:  http.StatusOK,
		},
		{
			name: "configuration with no labels should not error",
			admissionReq: createConfigurationAdmissionRequest(t, v1.Create, &Configuration{
				ObjectMeta: metav1.ObjectMeta{
					Labels: nil,
				},
				Spec: ConfigurationSpec{
					PackageSpec: PackageSpec{Package: "nginx"},
				},
			}),
			patch: []operations.PatchOperation{
				operations.ReplacePatchOperation(
					"/spec/packagePullSecrets",
					[]corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}},
				),
				operations.ReplacePatchOperation(
					"/spec/package",
					"127.0.0.1:31999/library/nginx:latest-zarf-3793515731",
				),
				operations.AddPatchOperation(
					"/metadata/labels",
					map[string]string{"zarf-agent": "patched"},
				),
			},
			code: http.StatusOK,
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
