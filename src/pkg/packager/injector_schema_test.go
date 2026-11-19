// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/value"
	corev1 "k8s.io/api/core/v1"
)

func TestInjectorValuesSchema_Validation(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("..", "..", "..", "packages", "init", "v1beta1", "injector", "values.schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	require.NoError(t, err, "values.schema.json should exist")

	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)

	t.Run("valid injector values pass schema", func(t *testing.T) {
		t.Parallel()
		validDoc := map[string]any{
			"injector": map[string]any{
				"nodeSelector": map[string]any{
					"topology.kubernetes.io/zone": "us-east-1a",
					"node-role.kubernetes.io/infra": "",
				},
				"tolerations": []any{
					map[string]any{
						"key":      "node-role.kubernetes.io/control-plane",
						"operator": "Exists",
						"effect":   "NoSchedule",
					},
					map[string]any{
						"key":      "dedicated",
						"operator": "Equal",
						"value":    "injector",
						"effect":   "NoExecute",
					},
				},
				"podTemplate": map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"sidecar.istio.io/inject": "false",
						},
					},
				},
				"serviceTemplate": map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]any{
							"service.beta.kubernetes.io/aws-load-balancer-internal": "true",
						},
					},
				},
			},
		}

		result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewGoLoader(validDoc))
		require.NoError(t, err)
		require.True(t, result.Valid(), "expected valid doc to pass schema validation: %v", result.Errors())
	})

	t.Run("invalid nodeSelector type fails schema", func(t *testing.T) {
		t.Parallel()
		invalidDoc := map[string]any{
			"injector": map[string]any{
				"nodeSelector": map[string]any{
					"invalid-int-value": 12345,
				},
			},
		}

		result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewGoLoader(invalidDoc))
		require.NoError(t, err)
		require.False(t, result.Valid(), "expected non-string nodeSelector to fail")
	})

	t.Run("invalid toleration operator fails schema", func(t *testing.T) {
		t.Parallel()
		invalidDoc := map[string]any{
			"injector": map[string]any{
				"tolerations": []any{
					map[string]any{
						"key":      "key",
						"operator": "InvalidOperator",
					},
				},
			},
		}

		result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewGoLoader(invalidDoc))
		require.NoError(t, err)
		require.False(t, result.Valid(), "expected invalid operator to fail")
	})

	t.Run("extra root property fails schema", func(t *testing.T) {
		t.Parallel()
		invalidDoc := map[string]any{
			"injector": map[string]any{},
			"unknownProperty": "notAllowed",
		}

		result, err := gojsonschema.Validate(schemaLoader, gojsonschema.NewGoLoader(invalidDoc))
		require.NoError(t, err)
		require.False(t, result.Valid(), "expected additional root property to fail")
	})
}

func TestInitWithInjectorValuesIntegration(t *testing.T) {
	t.Parallel()

	// Verify that values matching values.schema.json are resolved cleanly by deployer
	inputJSON := `{
		"injector": {
			"nodeSelector": {
				"node-type": "management"
			},
			"tolerations": [
				{
					"key": "node-role.kubernetes.io/control-plane",
					"operator": "Exists",
					"effect": "NoSchedule"
				}
			],
			"podTemplate": {
				"metadata": {
					"annotations": {
						"example.com/custom": "applied"
					}
				}
			},
			"serviceTemplate": {
				"metadata": {
					"labels": {
						"example.com/service-label": "custom"
					}
				}
			}
		}
	}`

	var vals value.Values
	err := json.Unmarshal([]byte(inputJSON), &vals)
	require.NoError(t, err)

	d := deployer{
		s: &state.State{
			InjectorInfo: state.InjectorInfo{Port: 32000},
			RegistryInfo: state.RegistryInfo{Port: 31999},
			IPFamily:     state.IPFamilyIPv4,
		},
		vals: vals,
	}

	opts, err := d.resolveInjectorOptions(DeployOptions{})
	require.NoError(t, err)

	require.Equal(t, map[string]string{"node-type": "management"}, opts.NodeSelector)
	require.Len(t, opts.Tolerations, 1)
	require.Equal(t, "node-role.kubernetes.io/control-plane", opts.Tolerations[0].Key)
	require.Equal(t, corev1.TolerationOpExists, opts.Tolerations[0].Operator)
	require.Equal(t, corev1.TaintEffectNoSchedule, opts.Tolerations[0].Effect)
	require.NotNil(t, opts.PodTemplate)
	require.Equal(t, "applied", opts.PodTemplate.Annotations["example.com/custom"])
	require.NotNil(t, opts.ServiceTemplate)
	require.Equal(t, "custom", opts.ServiceTemplate.Labels["example.com/service-label"])
}
