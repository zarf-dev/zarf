// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"encoding/json"
	"fmt"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
)

type Source struct {
	Path           string `json:"path"`
	RepoURL        string `json:"repoURL,omitempty"`
	TargetRevision string `json:"targetRevision,omitempty"`
}

type ArgoApplication struct {
	Spec struct {
		Source Source `json:"source"`
	}
}

// NewApplicationMutationHook creates a new instance of the ArgoCD Application mutation hook.
func NewApplicationMutationHook() operations.Hook {
	message.Debug("hooks.NewApplicationMutationHook()")
	return operations.Hook{
		Create: mutateApplication,
		Update: mutateApplication,
	}
}

// mutateGitRepoCreate mutates the git repository url to point to the repository URL defined in the ZarfState.
func mutateApplication(r *v1.AdmissionRequest) (result *operations.Result, err error) {

	var (
		zarfState types.ZarfState
		patches   []operations.PatchOperation
		isPatched bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	// Form the zarfState.GitServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	message.Debugf("Using the url of (%s) to mutate the ArgoCD Application", zarfState.GitServer.Address)

	// parse to simple struct to read the git url
	src := &ArgoApplication{}

	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}
	patchedURL := src.Spec.Source.RepoURL

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = utils.DoHostnamesMatch(zarfState.GitServer.Address, src.Spec.Source.RepoURL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the repoURL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitTransformURL(zarfState.GitServer.Address, patchedURL, zarfState.GitServer.PushUsername)
		if err != nil {
			message.Warnf("Unable to transform the repoURL, using the original url we have: %s", patchedURL)
		}
		patchedURL = transformedURL.String()
		message.Debugf("original repoURL of (%s) got mutated to (%s)", src.Spec.Source.RepoURL, patchedURL)
	}

	// Patch updates of the repo spec
	patches = populateArgoApplicationPatchOperations(patchedURL)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the Argo source spec.
func populateArgoApplicationPatchOperations(repoURL string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/source/repoURL", repoURL))

	return patches
}
