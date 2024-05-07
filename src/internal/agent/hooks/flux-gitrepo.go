// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
	v1 "k8s.io/api/admission/v1"
)

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook.
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateGitRepo,
		Update: mutateGitRepo,
	}
}

// mutateGitRepo mutates the GitRepo URL to point to the repository URL defined in the ZarfState.
func mutateGitRepo(r *v1.AdmissionRequest) (result *operations.Result, err error) {

	var (
		zarfState *types.ZarfState
		patches   []operations.PatchOperation
		isPatched bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	// Form the zarfState.GitServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	message.Debugf("Using the url of (%s) to mutate the flux GitRepository", zarfState.GitServer.Address)

	// Parse into a simple struct to read the GitRepo URL
	src := &flux.GitRepository{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}
	patchedURL := src.Spec.URL

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(zarfState.GitServer.Address, src.Spec.URL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the git URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(zarfState.GitServer.Address, patchedURL, zarfState.GitServer.PushUsername)
		if err != nil {
			message.Warnf("Unable to transform the GitRepo URL, using the original url we have: %s", patchedURL)
		}
		patchedURL = transformedURL.String()
		message.Debugf("original GitRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)
	}

	// Patch updates of the repo spec
	patches = populateGitRepoPatchOperations(patchedURL, src.ObjectMeta.Annotations)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateGitRepoPatchOperations(repoURL string, annotations map[string]string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	patches = addPatchedAnnotation(patches, annotations)

	// TODO, need to test that I don't need to check if the secret is already there
	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", meta.LocalObjectReference{Name: config.ZarfGitServerSecretName}))

	return patches
}
