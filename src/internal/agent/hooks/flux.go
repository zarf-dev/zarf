package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
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
		Create: mutateGitRepository,
		Update: mutateGitRepository,
	}
}

func mutateGitRepository(r *v1.AdmissionRequest) (*operations.Result, error) {
	var patches []operations.PatchOperation

	zarfState, err := getZarfStateFromFile(zarfStatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load zarf state from file: %v", err)
	}

	// Default to the InCluster gitURL
	gitServerURL := config.ZarfInClusterGitServiceURL

	// Check if we initialized with an external server
	if !zarfState.GitServerInfo.InternalServer {
		gitServerURL = zarfState.GitServerInfo.GitAddress

		if zarfState.GitServerInfo.GitPort != 0 {
			gitServerURL += fmt.Sprintf(":%d", zarfState.GitServerInfo.GitPort)
		}
	}

	message.Debugf("Using the gitServerURL of (%s) to mutate the flux repository", gitServerURL)

	// parse to simple struct to read the git url
	gitRepo := &GenericGitRepo{}
	if err := json.Unmarshal(r.Object.Raw, &gitRepo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %v", err)
	}

	message.Info(gitRepo.Spec.URL)

	replacedURL := git.MutateGitUrlsInText(gitServerURL, gitRepo.Spec.URL, zarfState.GitServerInfo.GitPushUsername)

	patches = append(patches, operations.ReplacePatchOperation("/spec/url", replacedURL))

	// If a prior secret exists, replace it
	if gitRepo.Spec.SecretRef.Name != "" {
		patches = append(patches, operations.ReplacePatchOperation("/spec/secretRef/name", config.ZarfGitServerSecretName))
	} else {
		// Otherwise, add the new secret
		patches = append(patches, operations.AddPatchOperation("/spec/secretRef", SecretRef{Name: config.ZarfGitServerSecretName}))
	}

	return &operations.Result{
		Allowed:  true,
		PatchOps: patches,
	}, nil
}

func getZarfStateFromFile(zarfStatePath string) (zarfState types.ZarfState, err error) {
	// Read the state file
	stateFile, err := ioutil.ReadFile(zarfStatePath)
	if err != nil {
		message.Warnf("Unable to read the zarfState file within the zarf-agent pod.")
		return zarfState, err
	}

	// Unmarshal the json file into a Go struct
	err = json.Unmarshal([]byte(stateFile), &zarfState)
	if err != nil {
		message.Warnf("Unable to umarshal the zarfState file into a useable object.")
		return zarfState, err
	}

	message.Debugf("ZarfState from file = %#v", zarfState)

	return zarfState, err
}
