// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"
)

// NewOCIRepositoryMutationHook creates a new instance of the oci repo mutation hook.
func NewOCIRepositoryMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateOCIRepo(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateOCIRepo(ctx, r, cluster)
		},
	}
}

// mutateOCIRepo mutates the oci repository url to point to the repository URL defined in the ZarfState.
func mutateOCIRepo(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	src := &flux.OCIRepository{}
	if err := json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	if src.Spec.Reference == nil {
		src.Spec.Reference = &flux.OCIRepositoryRef{}
	}

	// If we have a semver we want to continue since we wil still have the upstream tag
	// but should warn that we can't guarantee there won't be collisions
	if src.Spec.Reference.SemVer != "" {
		message.Warnf(lang.AgentWarnSemVerRef, src.Spec.Reference.SemVer)
	}

	if src.Labels != nil && src.Labels["zarf-agent"] == "patched" {
		return &operations.Result{
			Allowed:  true,
			PatchOps: []operations.PatchOperation{},
		}, nil
	}

	zarfState, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, err
	}

	// Get the registry service info if this is a NodePort service to use the internal kube-dns
	registryAddress, err := cluster.GetServiceInfoFromRegistryAddress(ctx, zarfState.RegistryInfo.Address)
	if err != nil {
		return nil, err
	}

	// For the internal registry this will be the ip & port of the service, it may look like 10.43.36.151:5000
	message.Debugf("Using the url of (%s) to mutate the flux OCIRepository", registryAddress)

	ref := src.Spec.URL
	if src.Spec.Reference.Digest != "" {
		ref = fmt.Sprintf("%s@%s", ref, src.Spec.Reference.Digest)
	} else if src.Spec.Reference.Tag != "" {
		ref = fmt.Sprintf("%s:%s", ref, src.Spec.Reference.Tag)
	}

	patchedSrc, err := transform.ImageTransformHost(registryAddress, ref)
	if err != nil {
		return nil, fmt.Errorf("unable to transform the OCIRepo URL: %w", err)
	}

	patchedRefInfo, err := transform.ParseImageRef(patchedSrc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the transformed OCIRepo URL: %w", err)
	}
	patchedRef := src.Spec.Reference

	patchedURL := helpers.OCIURLPrefix + patchedRefInfo.Name

	if patchedRefInfo.Digest != "" {
		patchedRef.Digest = patchedRefInfo.Digest
	} else if patchedRefInfo.Tag != "" {
		patchedRef.Tag = patchedRefInfo.Tag
	}

	message.Debugf("original OCIRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)

	patches := populateOCIRepoPatchOperations(patchedURL, zarfState.RegistryInfo.InternalRegistry, patchedRef)

	patches = append(patches, getLabelPatch(src.Labels))
	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func populateOCIRepoPatchOperations(repoURL string, isInternal bool, ref *flux.OCIRepositoryRef) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", meta.LocalObjectReference{Name: config.ZarfImagePullSecretName}))

	if isInternal {
		patches = append(patches, operations.ReplacePatchOperation("/spec/insecure", true))
	}

	// If semver is used we don't want to add the ":latest" tag + crc to the spec
	if ref.SemVer != "" {
		return patches
	}

	if ref.Tag != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/ref/tag", ref.Tag))
	}

	return patches
}
