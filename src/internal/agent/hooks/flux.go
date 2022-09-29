package hooks

import (
	"encoding/json"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	v1 "k8s.io/api/admission/v1"
)

const zarfStatePath = "/etc/zarf-state/state"

type SecretRef struct {
	Name string `json:"name"`
}

type GenericGitRepo struct {
	Spec struct {
		URL       string    `json:"url"`
		SecretRef SecretRef `json:"secretRef,omitempty"`
	}
}

// NewGitRepositoryMutationHook creates a new instance of the git repo mutation hook
func NewGitRepositoryMutationHook() operations.Hook {
	message.Debug("hooks.NewGitRepositoryMutationHook()")
	return operations.Hook{
		Create: mutateGitRepo,
		Update: mutateGitRepo,
	}
}

// mutateGitRepoCreate mutates the git repository url to point to the repository URL defined in the zarfState.
func mutateGitRepo(r *v1.AdmissionRequest) (*operations.Result, error) {
	var patches []operations.PatchOperation

	// Form the gitServerURL from the state
	zarfState, err := getStateFromAgentPod(zarfStatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load zarf state from file: %w", err)
	}
	gitServerURL := zarfState.GitServer.Address
	message.Debugf("Using the gitServerURL of (%s) to mutate the flux repository", gitServerURL)

	// parse to simple struct to read the git url
	gitRepo := &GenericGitRepo{}
	if err := json.Unmarshal(r.Object.Raw, &gitRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	gitURL := gitRepo.Spec.URL

	// Check if this is an update operation and the hostname is different from what we have in the state
	// NOTE: We mutate on updates IF AND ONLY IF the hostname in the request is different than the hostname in the zarfState
	// NOTE: We are checking if the hostname is different before because we do not want to potentially mutate a URL that has already been mutated.
	urlMatches := false
	if r.Operation == v1.Update {
		urlMatches, err = utils.DoesHostnamesMatch(gitServerURL, gitRepo.Spec.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to complete hostname matching: %w", err)
		}
	}

	// Mutate the git URL if necessary
	if r.Operation == v1.Create || (r.Operation == v1.Update && !urlMatches) {
		// Mutate the git URL so that the hostname matches the hostname in the Zarf state
		gitURL = git.MutateGitUrlsInText(gitServerURL, gitURL, zarfState.GitServer.PushUsername)
		message.Debugf("original git URL of (%s) got mutated to (%s)", gitRepo.Spec.URL, gitURL)
	}

	// Patch updates of the repo spec
	patches = populatePatchOperations(gitURL, gitRepo.Spec.SecretRef.Name)

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

// Patch updates of the repo spec.
func populatePatchOperations(repoURL string, secretName string) []operations.PatchOperation {
	var patches []operations.PatchOperation
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
