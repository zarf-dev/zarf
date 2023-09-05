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
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
)

// OCIRepo contains the URL of a git repo and the secret that corresponds to it for use with Flux.
type OCIRepo struct {
	Spec struct {
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
	} `json:"spec"`
}

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook.
func NewOCIRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewOCIRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateOCIRepo,
		Update: mutateOCIRepo,
	}
}

// mutateGitRepoCreate mutates the git repository url to point to the repository URL defined in the ZarfState.
func mutateOCIRepo(r *v1.AdmissionRequest) (result *operations.Result, err error) {

	var (
		zarfState     *types.ZarfState
		patches       []operations.PatchOperation
		newPatchedURL string
		isPatched     bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	// Form the zarfState.GitServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	message.Debugf("Using the url of (%s) to mutate the flux OCIRepository", zarfState.RegistryInfo.Address)

	// parse to simple struct to read the OCIRepo url
	src := &OCIRepo{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}
	patchedURL, err := removeOCIProtocol(src.Spec.URL)
	if err != nil {
		return nil, err
	}
	message.Debug("PatchedURL ", patchedURL)
	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		// check if image has already been transformed
		if strings.HasPrefix(zarfState.RegistryInfo.Address, patchedURL) {
			isPatched = true
		}
	}

	// Mutate the OCIRepo URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		newPatchedURL, err = transform.ImageTransformHost(zarfState.RegistryInfo.Address, patchedURL)
		if err != nil {
			message.Warnf("Unable to transform the OCIRepo URL, using the original url we have: %s", patchedURL)
		}

		message.Debugf("original OCIRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)
	}

	// Patch updates of the repo spec (Flux resource requires oci:// prefix)
	patches = populateOCIRepoPatchOperations(fmt.Sprintf("%s%s", "oci://", newPatchedURL), src.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateOCIRepoPatchOperations(repoURL string, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	message.Debug("in populateOCIRepoPatchOperations repoURL ", repoURL)
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	// If a prior secret exists, replace it
	if secretName != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfGitServerSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfGitServerSecretName}))
	}

	return patches
}
