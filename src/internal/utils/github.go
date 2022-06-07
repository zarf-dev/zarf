package utils

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/message"
)

// GetLatestReleaseTag returns the latest release tag for the given github project
func GetLatestReleaseTag(project string) (string, error) {
	message.Debugf("utils.GetLatestReleaseTag(%s)", project)

	// We only need the tag from the JSON response
	// See https://github.com/google/go-github/blob/v45.1.0/github/repos_releases.go#L21 for complete reference
	ghRelease := struct {
		TagName string `json:"tag_name"`
	}{}

	// Get the latest release from the GitHub API for the given project
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", project))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Decode the JSON response into the ghRelease struct
	if err := json.NewDecoder(resp.Body).Decode(&ghRelease); err != nil {
		return "", err
	}

	return ghRelease.TagName, nil
}
