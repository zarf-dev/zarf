// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
	v1 "k8s.io/api/admission/v1"
)

// NewHelmRepositoryMutationHook creates a new instance of the helm repo mutation hook.
func NewHelmRepositoryMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	message.Debug("hooks.NewHelmRepositoryMutationHook()")
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
func mutateHelmRepo(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (result *operations.Result, err error) {
	var (
		zarfState *types.ZarfState
		patches   []operations.PatchOperation
	)

	src := &flux.HelmRepository{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	// If we see a type of helm repo other than OCI we should flag a warning and return
	if strings.ToLower(src.Spec.Type) != "oci" {
		message.Warnf(lang.AgentWarnNotOCIType, src.Spec.Type)
		return &operations.Result{Allowed: true}, nil
	}

	if src.Annotations != nil && src.Annotations["zarf-agent"] == "patched" {
		return &operations.Result{
			Allowed:  true,
			PatchOps: patches,
		}, nil
	}

	if zarfState, err = cluster.LoadZarfState(ctx); err != nil {
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
		message.Warnf("Unable to transform the HelmRepo URL, using the original url we have: %s", src.Spec.URL)
		return &operations.Result{Allowed: true}, nil
	}

	patchedRefInfo, err := transform.ParseImageRef(patchedSrc)
	if err != nil {
		message.Warnf("Unable to parse the transformed HelmRepo URL, using the original url we have: %s", src.Spec.URL)
		return &operations.Result{Allowed: true}, nil
	}
	patchedURL := helpers.OCIURLPrefix + patchedRefInfo.Name

	message.Debugf("original HelmRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)

	// Patch updates of the repo spec (Flux resource requires oci:// prefix)
	patches = populateHelmRepoPatchOperations(patchedURL, zarfState.RegistryInfo.InternalRegistry)

	patches = append(patches, getAnnotationPatch(src.Annotations))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateHelmRepoPatchOperations(repoURL string, isInternal bool) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	if isInternal {
		patches = append(patches, operations.ReplacePatchOperation("/spec/insecure", true))
	}

	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", meta.LocalObjectReference{Name: config.ZarfImagePullSecretName}))

	return patches
}
