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
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
)

// HelmRepo contains the URL of a helm repo and the secret that corresponds to it for use with Flux.
type HelmRepo struct {
	Spec struct {
		Type      string    `json:"type"`
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
	} `json:"spec"`
}

// NewHelmRepositoryMutationHook creates a new instance of the helm repo mutation hook.
func NewHelmRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateHelmRepo,
		Update: mutateHelmRepo,
	}
}

// mutateHelmRepo mutates the repository url to point to the repository URL defined in the ZarfState.
func mutateHelmRepo(r *v1.AdmissionRequest) (result *operations.Result, err error) {
	message.Warn("HELM REPOMUTATE")
	var (
		zarfState     *types.ZarfState
		patches       []operations.PatchOperation
		isPatched     bool
		repoURL       string
		newPatchedURL string
		isCreate      = r.Operation == v1.Create
		isUpdate      = r.Operation == v1.Update
	)

	// Form the zarfState.GitServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	// Mutate the oci URL so that the hostname matches the hostname in the Zarf state
	// Must be valid DNS https://fluxcd.io/flux/components/source/helmrepositories/#writing-a-helmrepository-spec

	registryInternalURL := zarfState.RegistryInfo.Address
	registryServiceInfo, err := cluster.ServiceInfoFromNodePortURL(zarfState.RegistryInfo.Address)
	if err != nil {
		message.WarnErrorf(err, lang.WarnUnableToGetServiceInfo, "registry", zarfState.RegistryInfo.Address)
	} else {
		registryInternalURL = fmt.Sprintf("%s.%s.svc.cluster.local:%d", registryServiceInfo.Name, registryServiceInfo.Namespace, registryServiceInfo.Port)
	}

	message.Debugf("Using the url of (%s) to mutate the flux HelmRepository", registryInternalURL)

	// parse to simple struct to read the HelmRepo url
	src := &HelmRepo{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	if strings.ToLower(src.Spec.Type) != "oci" {
		message.Warnf(lang.AgentWarningNotOCIType, src.Spec.Type)
		return nil, nil
	}

	patchedURL, err := removeOCIProtocol(src.Spec.URL)
	if err != nil {
		return nil, err
	}
	// necessary so it doesn't double mutate
	newPatchedURL = patchedURL

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched = strings.Contains(patchedURL, registryInternalURL)
	}

	// Mutate the HelmRepo URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		newPatchedURL, err = transform.ImageTransformHostWithoutChecksumOrTag(registryInternalURL, patchedURL)
		if err != nil {
			message.Warnf(lang.WarnUnableToTransform, "HelmRepo", patchedURL)
		}

		message.Debugf("original HelmRepo URL of (%s) got mutated to (%s)", src.Spec.URL, newPatchedURL)
	}

	repoURL = fmt.Sprintf("%s%s", "oci://", newPatchedURL)

	// Patch updates of the repo spec (Flux resource requires oci:// prefix)
	patches = populateHelmRepoPatchOperations(repoURL, src.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateHelmRepoPatchOperations(repoURL string, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	// If a prior secret exists, replace it
	if secretName != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfImagePullSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfImagePullSecretName}))
	}

	return patches
}
