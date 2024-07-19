// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks provides HTTP handlers for the mutating webhook.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"

	corev1 "k8s.io/api/core/v1"
)

// NewPodMutationHook creates a new instance of pods mutation hook.
func NewPodMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutatePod(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutatePod(ctx, r, cluster)
		},
	}
}

func parsePod(object []byte) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := json.Unmarshal(object, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

func mutatePod(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	pod, err := parsePod(r.Object.Raw)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrParsePod, err)
	}

	if pod.Labels != nil && pod.Labels["zarf-agent"] == "patched" {
		// We've already played with this pod, just keep swimming 🐟
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

	var patches []operations.PatchOperation

	// Add the zarf secret to the podspec
	zarfSecret := []corev1.LocalObjectReference{{Name: config.ZarfImagePullSecretName}}
	patches = append(patches, operations.ReplacePatchOperation("/spec/imagePullSecrets", zarfSecret))

	// update the image host for each init container
	for idx, container := range pod.Spec.InitContainers {
		path := fmt.Sprintf("/spec/initContainers/%d/image", idx)
		replacement, err := transform.ImageTransformHost(registryURL, container.Image)
		if err != nil {
			message.Warnf(lang.AgentErrImageSwap, container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patches = append(patches, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each ephemeral container
	for idx, container := range pod.Spec.EphemeralContainers {
		path := fmt.Sprintf("/spec/ephemeralContainers/%d/image", idx)
		replacement, err := transform.ImageTransformHost(registryURL, container.Image)
		if err != nil {
			message.Warnf(lang.AgentErrImageSwap, container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patches = append(patches, operations.ReplacePatchOperation(path, replacement))
	}

	// update the image host for each normal container
	for idx, container := range pod.Spec.Containers {
		path := fmt.Sprintf("/spec/containers/%d/image", idx)
		replacement, err := transform.ImageTransformHost(registryURL, container.Image)
		if err != nil {
			message.Warnf(lang.AgentErrImageSwap, container.Image)
			continue // Continue, because we might as well attempt to mutate the other containers for this pod
		}
		patches = append(patches, operations.ReplacePatchOperation(path, replacement))
	}

	patches = append(patches, getLabelPatch(pod.Labels))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}
