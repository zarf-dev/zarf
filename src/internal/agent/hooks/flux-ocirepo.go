// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
)

// Ref contains the tag used to reference am image.
type Ref struct {
	Tag string `json:"tag,omitempty"`
}

// OCIRepo contains the URL of a git repo and the secret that corresponds to it for use with Flux.
type OCIRepo struct {
	Spec struct {
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
		Ref       Ref       `json:"ref,omitempty"`
	} `json:"spec"`
}

// NewOCIRepositoryMutationHook creates a new instance of the oci repo mutation hook.
func NewOCIRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewOCIRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateOCIRepo,
		Update: mutateOCIRepo,
	}
}

// mutateOCIRepo mutates the oci repository url to point to the repository URL defined in the ZarfState.
func mutateOCIRepo(r *v1.AdmissionRequest) (result *operations.Result, err error) {
	var (
		zarfState *types.ZarfState
		patches   []operations.PatchOperation
		isPatched bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	// Form the zarfState.RegistryServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	// Mutate the oci URL so that the hostname matches the hostname in the Zarf state
	// Must be valid DNS https://fluxcd.io/flux/components/source/ocirepositories/#writing-an-ocirepository-spec
	registryAddress := zarfState.RegistryInfo.Address
	c, err := cluster.NewCluster()
	if err != nil {
		return nil, fmt.Errorf(lang.WarnUnableToGetServiceInfo, "registry", zarfState.RegistryInfo.Address)
	}
	registryServiceInfo, err := c.ServiceInfoFromNodePortURL(zarfState.RegistryInfo.Address)
	if err != nil {
		message.Warnf(lang.WarnUnableToGetServiceInfo, "registry", zarfState.RegistryInfo.Address)
	} else {
		registryAddress = fmt.Sprintf("%s.%s.svc.cluster.local:%d", registryServiceInfo.Name, registryServiceInfo.Namespace, registryServiceInfo.Port)
	}

	message.Debugf("Using the url of (%s) to mutate the flux OCIRepository", registryAddress)

	// parse to simple struct to read the OCIRepo url
	src := &OCIRepo{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}
	patchedURL := src.Spec.URL
	patchedTag := src.Spec.Ref.Tag

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(registryAddress, src.Spec.URL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the OCIRepo URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		trimmedSrc := strings.TrimPrefix(src.Spec.URL, helpers.OCIURLPrefix)
		patchedSrc, err := transform.ImageTransformHost(registryAddress, trimmedSrc)
		if err != nil {
			message.Warnf(lang.WarnUnableToTransform, "OCIRepo", patchedSrc)
		}

		patchedRefInfo, err := transform.ParseImageRef(patchedSrc)
		if err != nil {
			message.Warnf(lang.WarnUnableToTransform, "OCIRepo", patchedSrc)
		}
		patchedURL = helpers.OCIURLPrefix + patchedRefInfo.Name
		patchedTag = patchedRefInfo.TagOrDigest

		message.Debugf("original OCIRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)
	}

	// Patch updates of the repo spec (Flux resource requires oci:// prefix)
	// repoURL := cluster.FetchInternalRegistryKubeDNSName()
	patches = populateOCIRepoPatchOperations(patchedURL, patchedTag, src.Spec.SecretRef.Name)
	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateOCIRepoPatchOperations(repoURL, repoTag, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	// If a prior secret exists, replace it
	if secretName != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfImagePullSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfImagePullSecretName}))
	}

	if repoTag != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/ref/tag", repoTag))
	}

	return patches
}
