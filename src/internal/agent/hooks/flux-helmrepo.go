// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"
)

// NewHelmRepositoryMutationHook creates a new instance of the helm repo mutation hook.
func NewHelmRepositoryMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateHelmRepo(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateHelmRepo(ctx, r, cluster)
		},
	}
}

// mutateHelmRepo mutates the repository url to point to the repository URL defined in the ZarfState.
func mutateHelmRepo(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	src := &flux.HelmRepository{}
	if err := json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	// If we see a type of helm repo other than OCI we should flag a warning and return
	if strings.ToLower(src.Spec.Type) != "oci" {
		message.Warnf(lang.AgentWarnNotOCIType, src.Spec.Type)
		return &operations.Result{Allowed: true}, nil
	}

	if src.Labels != nil && src.Labels["zarf-agent"] == "patched" {
		return &operations.Result{
			Allowed:  true,
			PatchOps: nil,
		}, nil
	}

	zarfState, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	// Get the registry service info if this is a NodePort service to use the internal kube-dns
	registryAddress, err := cluster.GetServiceInfoFromRegistryAddress(ctx, zarfState.RegistryInfo.Address)
	if err != nil {
		return nil, err
	}

	message.Debugf("Using the url of (%s) to mutate the flux HelmRepository", registryAddress)

	patchedSrc, err := transform.ImageTransformHost(registryAddress, src.Spec.URL)
	if err != nil {
		return nil, fmt.Errorf("unable to transform the HelmRepo URL: %w", err)
	}

	patchedRefInfo, err := transform.ParseImageRef(patchedSrc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the HelmRepo URL: %w", err)
	}
	patchedURL := helpers.OCIURLPrefix + patchedRefInfo.Name

	message.Debugf("original HelmRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)

	patches := populateHelmRepoPatchOperations(patchedURL, zarfState.RegistryInfo.IsInternal())

	patches = append(patches, getLabelPatch(src.Labels))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func populateHelmRepoPatchOperations(repoURL string, isInternal bool) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	if isInternal {
		patches = append(patches, operations.ReplacePatchOperation("/spec/insecure", true))
	}

	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", meta.LocalObjectReference{Name: config.ZarfImagePullSecretName}))

	return patches
}
