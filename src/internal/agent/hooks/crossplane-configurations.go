// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks provides HTTP handlers for the mutating webhook.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"

	corev1 "k8s.io/api/core/v1"
)

// NewConfigurationMutationHook creates a new instance of configurations mutation hook.
func NewConfigurationMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	message.Debug("hooks.NewMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateConfiguration(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateConfiguration(ctx, r, cluster)
		},
	}
}

func parseConfiguration(object []byte) (*Configuration, error) {
	message.Debugf("configurations.parseConfiguration(%s)", string(object))
	var configuration Configuration
	if err := json.Unmarshal(object, &configuration); err != nil {
		return nil, err
	}
	return &configuration, nil
}

func mutateConfiguration(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	message.Debugf("hooks.mutateConfiguration()(*v1.AdmissionRequest) - %#v , %s/%s: %#v", r.Kind, r.Namespace, r.Name, r.Operation)

	configuration, err := parseConfiguration(r.Object.Raw)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrParseConfiguration, err)
	}

	if configuration.Labels != nil && configuration.Labels["zarf-agent"] == "patched" {
		// We've already played with this configuration, just keep swimming 🐟
		return &operations.Result{
			Allowed:  true,
			PatchOps: []operations.PatchOperation{},
		}, nil
	}

	state, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}
	registryURL := state.RegistryInfo.Address

	var patchOperations []operations.PatchOperation

	// Add the zarf secret to the configurationspec
	zarfSecret := []corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}}
	patchOperations = append(patchOperations, operations.ReplacePatchOperation("/spec/packagePullSecrets", zarfSecret))

	// update the package host for the configuration
	replacement, err := transform.ImageTransformHost(registryURL, configuration.Spec.Package)
	if err != nil {
		message.Warnf(lang.AgentErrImageSwap, configuration.Spec.Package)
	}
	patchOperations = append(patchOperations, operations.ReplacePatchOperation("/spec/package", replacement))

	// Add a label noting the zarf mutation
	if configuration.Labels == nil {
		// If the labels path does not exist - create with map[string]string value
		patchOperations = append(patchOperations, operations.AddPatchOperation("/metadata/labels",
			map[string]string{
				"zarf-agent": "patched",
			}))
	} else {
		patchOperations = append(patchOperations, operations.ReplacePatchOperation("/metadata/labels/zarf-agent", "patched"))
	}

	return &operations.Result{
		Allowed:  true,
		PatchOps: patchOperations,
	}, nil
}
