package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaAndGrafana(t *testing.T) {
	t.Log("E2E: Testing gitea and grafana")

	// Deploy the gitops example
	path := fmt.Sprintf("build/zarf-package-gitops-service-data-%s.tar.zst", e2e.arch)
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)

	// Establish the port-forward into the gitea service; give the service a few seconds to come up since this is not a command we can retry
	err = e2e.execZarfBackgroundCommand("connect", "git", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to git")

	// Make sure Gitea comes up cleanly
	resp, err := http.Get("http://127.0.0.1:45003/explore/repos")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Establish the port-forward into the logging service
	err = e2e.execZarfBackgroundCommand("connect", "logging", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to logging")

	// Make sure Grafana comes up cleanly
	resp, err = http.Get("http://127.0.0.1:45002/monitor/login")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Init the state variable
	state := k8s.LoadZarfState()
	config.InitState(state)

	// Get the repo as the readonly user
	repoName := "mirror__repo1.dso.mil__platform-one__big-bang__apps__security-tools__twistlock"
	getRepoRequest, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/api/v1/repos/%v/%v", config.IPV4Localhost, k8s.PortGit, config.ZarfGitPushUser, repoName), nil)
	getRepoResponseBody, err := git.DoHttpThings(getRepoRequest, config.ZarfGitReadUser, config.GetSecret(config.StateGitPull))
	assert.NoError(t, err)

	// Make sure the only permissions are pull (read)
	var bodyMap map[string]interface{}
	json.Unmarshal(getRepoResponseBody, &bodyMap)
	permissionsMap := bodyMap["permissions"].(map[string]interface{})
	assert.False(t, permissionsMap["admin"].(bool))
	assert.False(t, permissionsMap["push"].(bool))
	assert.True(t, permissionsMap["pull"].(bool))
}
