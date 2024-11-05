// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application is a definition of an ArgoCD Application resource.
// The ArgoCD Application structs in this file have been partially copied from upstream.
//
// https://github.com/argoproj/argo-cd/blob/v2.11.0/pkg/apis/application/v1alpha1/types.go
//
// There were errors encountered when trying to import argocd as a Go package.
//
// For more information: https://argo-cd.readthedocs.io/en/stable/user-guide/import/
type Application struct {
	Spec ApplicationSpec `json:"spec"`
	metav1.ObjectMeta
}

// ApplicationSpec represents desired application state. Contains link to repository with application definition.
type ApplicationSpec struct {
	// Source is a reference to the location of the application's manifests or chart.
	Source  *ApplicationSource  `json:"source,omitempty"`
	Sources []ApplicationSource `json:"sources,omitempty"`
}

// ApplicationSource contains all required information about the source of an application.
type ApplicationSource struct {
	// RepoURL is the URL to the repository (Git or Helm) that contains the application manifests.
	RepoURL string `json:"repoURL"`
}

// NewApplicationMutationHook creates a new instance of the ArgoCD Application mutation hook.
func NewApplicationMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateApplication(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateApplication(ctx, r, cluster)
		},
	}
}

// mutateApplication mutates the git repository url to point to the repository URL defined in the ZarfState.
func mutateApplication(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (*operations.Result, error) {
	l := logger.From(ctx)
	state, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, err
	}

	app := Application{}
	if err = json.Unmarshal(r.Object.Raw, &app); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	l.Info("using the Zarf git server URL to mutate the ArgoCD Application repo URL(s)",
		"resource", app.Name,
		"git-server", state.GitServer.Address)

	patches := make([]operations.PatchOperation, 0)
	if app.Spec.Source != nil {
		patchedURL, err := getPatchedRepoURL(ctx, app.Spec.Source.RepoURL, state.GitServer, r)
		if err != nil {
			return nil, err
		}
		patches = populateSingleSourceArgoApplicationPatchOperations(patchedURL, patches)
	}

	if len(app.Spec.Sources) > 0 {
		for idx, source := range app.Spec.Sources {
			patchedURL, err := getPatchedRepoURL(ctx, source.RepoURL, state.GitServer, r)
			if err != nil {
				return nil, err
			}
			patches = populateMultipleSourceArgoApplicationPatchOperations(idx, patchedURL, patches)
		}
	}

	patches = append(patches, getLabelPatch(app.Labels))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func getPatchedRepoURL(ctx context.Context, repoURL string, gs types.GitServerInfo, r *v1.AdmissionRequest) (string, error) {
	l := logger.From(ctx)
	isCreate := r.Operation == v1.Create
	isUpdate := r.Operation == v1.Update
	patchedURL := repoURL
	var isPatched bool
	var err error

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different from the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(gs.Address, repoURL)
		if err != nil {
			return "", fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	// Mutate the repoURL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(gs.Address, patchedURL, gs.PushUsername)
		if err != nil {
			return "", fmt.Errorf("%s: %w", AgentErrTransformGitURL, err)
		}
		patchedURL = transformedURL.String()
		l.Debug("mutated ArgoCD application repoURL to the Zarf URL", "original", repoURL, "mutated", patchedURL)
	}

	return patchedURL, nil
}

// Patch updates of the Argo source spec.
func populateSingleSourceArgoApplicationPatchOperations(repoURL string, patches []operations.PatchOperation) []operations.PatchOperation {
	return append(patches, operations.ReplacePatchOperation("/spec/source/repoURL", repoURL))
}

// Patch updates of the Argo sources spec.
func populateMultipleSourceArgoApplicationPatchOperations(idx int, repoURL string, patches []operations.PatchOperation) []operations.PatchOperation {
	return append(patches, operations.ReplacePatchOperation(fmt.Sprintf("/spec/sources/%d/repoURL", idx), repoURL))
}
