// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
)

// RepoCreds holds the definition for repository credentials.
// This has been partially copied from upstream.
//
// https://github.com/argoproj/argo-cd/blob/v2.11.0/pkg/apis/application/v1alpha1/repository_types.go
//
// There were errors encountered when trying to import argocd as a Go package.
//
// For more information: https://argo-cd.readthedocs.io/en/stable/user-guide/import/
type RepoCreds struct {
	// URL is the URL that this credential matches to.
	URL string `json:"url"`
}

// NewRepositorySecretMutationHook creates a new instance of the ArgoCD repository secret mutation hook.
func NewRepositorySecretMutationHook(ctx context.Context, cluster *cluster.Cluster) operations.Hook {
	message.Debug("hooks.NewRepositoryMutationHook()")
	return operations.Hook{
		Create: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateRepositorySecret(ctx, r, cluster)
		},
		Update: func(r *v1.AdmissionRequest) (*operations.Result, error) {
			return mutateRepositorySecret(ctx, r, cluster)
		},
	}
}

// mutateRepositorySecret mutates the git URL in the ArgoCD repository secret to point to the repository URL defined in the ZarfState.
func mutateRepositorySecret(ctx context.Context, r *v1.AdmissionRequest, cluster *cluster.Cluster) (result *operations.Result, err error) {
	isCreate := r.Operation == v1.Create
	isUpdate := r.Operation == v1.Update
	var isPatched bool

	state, err := cluster.LoadZarfState(ctx)
	if err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	message.Infof("Using the url of (%s) to mutate the ArgoCD Repository Secret", state.GitServer.Address)

	secret := corev1.Secret{}
	if err = json.Unmarshal(r.Object.Raw, &secret); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	url, exists := secret.Data["url"]
	if !exists {
		return nil, fmt.Errorf("url field not found in argocd repository secret data")
	}

	var repoCreds RepoCreds
	repoCreds.URL = string(url)

	// Check if this is an update operation and the hostname is different from what we have in the zarfState
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different from the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	if isUpdate {
		isPatched, err = helpers.DoHostnamesMatch(state.GitServer.Address, repoCreds.URL)
		if err != nil {
			return nil, fmt.Errorf(lang.AgentErrHostnameMatch, err)
		}
	}

	patchedURL := repoCreds.URL
	// Mutate the repoURL if necessary
	if isCreate || (isUpdate && !isPatched) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		transformedURL, err := transform.GitURL(state.GitServer.Address, repoCreds.URL, state.GitServer.PushUsername)
		if err != nil {
			return nil, fmt.Errorf("unable the git url: %w", err)
		}
		patchedURL = transformedURL.String()
		message.Debugf("original url of (%s) got mutated to (%s)", repoCreds.URL, patchedURL)
	}

	patches := populateArgoRepositoryPatchOperations(patchedURL, state.GitServer)
	patches = append(patches, getLabelPatch(secret.Labels))

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the Argo Repository Secret.
func populateArgoRepositoryPatchOperations(repoURL string, gitServer types.GitServerInfo) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/data/url", base64.StdEncoding.EncodeToString([]byte(repoURL))))
	patches = append(patches, operations.ReplacePatchOperation("/data/username", base64.StdEncoding.EncodeToString([]byte(gitServer.PullUsername))))
	patches = append(patches, operations.ReplacePatchOperation("/data/password", base64.StdEncoding.EncodeToString([]byte(gitServer.PullPassword))))

	return patches
}
