// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/agent/state"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Source represents a subset of the Argo Source object needed for Zarf Git URL mutations
type Source struct {
	RepoURL string `json:"repoURL"`
}

// ArgoApplication represents a subset of the Argo Application object needed for Zarf Git URL mutations
type ArgoApplication struct {
	Spec struct {
		Source  Source   `json:"source"`
		Sources []Source `json:"sources"`
	} `json:"spec"`
	metav1.ObjectMeta
}

var (
	zarfState *types.ZarfState
	isPatched bool
	isCreate  bool
	isUpdate  bool
)

// NewApplicationMutationHook creates a new instance of the ArgoCD Application mutation hook.
func NewApplicationMutationHook() operations.Hook {
	message.Debug("hooks.NewApplicationMutationHook()")
	return operations.Hook{
		Create: mutateApplication,
		Update: mutateApplication,
	}
}

// mutateApplication mutates the git repository url to point to the repository URL defined in the ZarfState.
func mutateApplication(r *v1.AdmissionRequest) (result *operations.Result, err error) {

	isCreate = r.Operation == v1.Create
	isUpdate = r.Operation == v1.Update

	patches := []operations.PatchOperation{}

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

	message.Debugf("Data %v", string(r.Object.Raw))

	if src.Spec.Source != (Source{}) {
		patchedURL, _ := getPatchedRepoURL(src.Spec.Source.RepoURL)
		patches = populateSingleSourceArgoApplicationPatchOperations(patchedURL, patches)
	}

	if len(src.Spec.Sources) > 0 {
		for idx, source := range src.Spec.Sources {
			patchedURL, _ := getPatchedRepoURL(source.RepoURL)
			patches = populateMultipleSourceArgoApplicationPatchOperations(idx, patchedURL, patches)
		}
	}

	patches = addPatchedAnnotation(patches, src.Annotations)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func getPatchedRepoURL(repoURL string) (string, error) {
	var err error
	patchedURL := repoURL

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different from the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(zarfState.GitServer.Address, repoURL)
		if err != nil {
			return "", fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the repoURL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(zarfState.GitServer.Address, patchedURL, zarfState.GitServer.PushUsername)
		if err != nil {
			message.Warnf("Unable to transform the repoURL, using the original url we have: %s", patchedURL)
		}
		patchedURL = transformedURL.String()
		message.Debugf("original repoURL of (%s) got mutated to (%s)", repoURL, patchedURL)
	}

	return patchedURL, err
}

// Patch updates of the Argo source spec.
func populateSingleSourceArgoApplicationPatchOperations(repoURL string, patches []operations.PatchOperation) []operations.PatchOperation {
	return append(patches, operations.ReplacePatchOperation("/spec/source/repoURL", repoURL))
}

// Patch updates of the Argo sources spec.
func populateMultipleSourceArgoApplicationPatchOperations(idx int, repoURL string, patches []operations.PatchOperation) []operations.PatchOperation {
	return append(patches, operations.ReplacePatchOperation(fmt.Sprintf("/spec/sources/%d/repoURL", idx), repoURL))
}
