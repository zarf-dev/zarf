// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	flux "github.com/fluxcd/source-controller/api/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	v1 "k8s.io/api/admission/v1"
)

// AgentErrTransformGitURL is thrown when the agent fails to make the git url a Zarf compatible url
const AgentErrTransformGitURL = "unable to transform the git url"

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook.
func NewGitRepositoryMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
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
func mutateGitRepo(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	l := logger.From(ctx)
	var (
		patches   []operations.PatchOperation
		isPatched bool

		isCreate = r.Operation == v1.Create
		isUpdate = r.Operation == v1.Update
	)

	state, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, err
	}

	repo := flux.GitRepository{}
	if err = json.Unmarshal(r.Object.Raw, &repo); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	l.Info("using the Zarf git server URL to mutate the Flux GitRepository",
		"name", repo.Name,
		"git-server", state.GitServer.Address)

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(state.GitServer.Address, repo.Spec.URL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	patchedURL := repo.Spec.URL

	// Mutate the git URL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(state.GitServer.Address, patchedURL, state.GitServer.PushUsername)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", AgentErrTransformGitURL, err)
		}
		patchedURL = transformedURL.String()
		l.Debug("mutating the Flux GitRepository URL to the Zarf URL", "original", repo.Spec.URL, "mutated", patchedURL)
	}

	// Patch updates of the repo spec
	patches = populatePatchOperations(patchedURL)
	patches = append(patches, getLabelPatch(repo.Labels))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populatePatchOperations(repoURL string) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	newSecretRef := fluxmeta.LocalObjectReference{Name: config.ZarfGitServerSecretName}
	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", newSecretRef))

	return patches
}
