// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"
)

// SecretRef contains the name used to reference a git repository secret.
type SecretRef struct {
	Name string `json:"name"`
}

// GenericGitRepo contains the URL of a git repo and the secret that corresponds to it for use with Flux.
type GenericGitRepo struct {
	Spec struct {
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
	} `json:"spec"`
}

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook.
func NewGitRepositoryMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateGitRepo(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateGitRepo(ctx, r, cluster)
		},
	}
}

// mutateGitRepoCreate mutates the git repository url to point to the repository URL defined in the ZarfState.
func mutateGitRepo(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (result *operations.Result, err error) {

	var (
		patches   []operations.PatchOperation
		isPatched bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	// Form the zarfState.GitServer.Address from the zarfState

	state, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	message.Debugf("Using the url of (%s) to mutate the flux repository", state.GitServer.Address)

	// parse to simple struct to read the git url
	src := &GenericGitRepo{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}
	patchedURL := src.Spec.URL

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(state.GitServer.Address, src.Spec.URL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the git URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(state.GitServer.Address, patchedURL, state.GitServer.PushUsername)
		if err != nil {
			message.Warnf("Unable to transform the git url, using the original url we have: %s", patchedURL)
		}
		patchedURL = transformedURL.String()
		message.Debugf("original git URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)
	}

	// Patch updates of the repo spec
	patches = populatePatchOperations(patchedURL, src.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populatePatchOperations(repoURL string, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	//TODO This logic can be simplified

	// If a prior secret exists, replace it
	if secretName != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfGitServerSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfGitServerSecretName}))
	}

	return patches
}
