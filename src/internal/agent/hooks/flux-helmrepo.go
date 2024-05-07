// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"encoding/json"
	"fmt"
	"strings"

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

// NewHelmRepositoryMutationHook creates a new instance of the helm repo mutation hook.
func NewHelmRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewHelmRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateHelmRepo,
		Update: mutateHelmRepo,
	}
}

// mutateHelmRepo mutates the repository url to point to the repository URL defined in the ZarfState.
func mutateHelmRepo(r *v1.AdmissionRequest) (result *operations.Result, err error) {
	var (
		zarfState *types.ZarfState
		patches   []operations.PatchOperation
	)

	// Parse into a simple struct to read the HelmRepo url
	src := &flux.HelmRepository{}
	if err = json.Unmarshal(r.Object.Raw, &src); err != nil {
		return nil, fmt.Errorf(lang.ErrUnmarshal, err)
	}

	// If we see a type of helm repo other than OCI we should flag a warning and return
	if strings.ToLower(src.Spec.Type) != "oci" {
		message.Warnf(lang.AgentWarnNotOCIType, src.Spec.Type)
		return &operations.Result{Allowed: true}, nil
	}

	// Note: for HelmRepositories we only patch the URL because the tag and Chart version are coupled together.
	if src.Labels != nil && src.Labels["zarf-agent"] == "patched" {
		message.Debugf("We are now in this object %v", src.ObjectMeta)
		return &operations.Result{
			Allowed:  true,
			PatchOps: patches,
		}, nil
	}

	// Form the zarfState.GitServer.Address from the zarfState
	if zarfState, err = state.GetZarfStateFromAgentPod(); err != nil {
		return nil, fmt.Errorf(lang.AgentErrGetState, err)
	}

	// Get the registry service info if this is a NodePort service to use the internal kube-dns
	registryAddress, err := state.GetServiceInfoFromRegistryAddress(zarfState.RegistryInfo.Address)
	if err != nil {
		return nil, err
	}

	message.Debugf("Using the url of (%s) to mutate the flux HelmRepository", registryAddress)

	patchedSrc, err := transform.ImageTransformHost(registryAddress, src.Spec.URL)
	if err != nil {
		message.Warnf("Unable to transform the HelmRepo URL, using the original url we have: %s", src.Spec.URL)
		return &operations.Result{Allowed: true}, nil
	}

	patchedRefInfo, err := transform.ParseImageRef(patchedSrc)
	if err != nil {
		message.Warnf("Unable to parse the transformed HelmRepo URL, using the original url we have: %s", src.Spec.URL)
		return &operations.Result{Allowed: true}, nil
	}
	patchedURL := helpers.OCIURLPrefix + patchedRefInfo.Name

	message.Debugf("original HelmRepo URL of (%s) got mutated to (%s)", src.Spec.URL, patchedURL)

	// Patch updates of the repo spec (Flux resource requires oci:// prefix)
	patches = populateHelmRepoPatchOperations(patchedURL, zarfState.RegistryInfo.InternalRegistry)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populateHelmRepoPatchOperations(repoURL string, isInternal bool) []operations.PatchOperation {
	var patches []operations.PatchOperation
	patches = append(patches, operations.ReplacePatchOperation("/spec/url", repoURL))

	if isInternal {
		patches = append(patches, operations.ReplacePatchOperation("/spec/insecure", true))
	}

	patches = append(patches, operations.ReplacePatchOperation("/metadata/annotations/zarf-agent", "patched"))

	patches = append(patches, operations.AddPatchOperation("/spec/secretRef", meta.LocalObjectReference{Name: config.ZarfImagePullSecretName}))

	return patches
}
