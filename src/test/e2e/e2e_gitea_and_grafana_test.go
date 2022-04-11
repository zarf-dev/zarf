package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaAndGrafana(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	// run `zarf init`
	output, err := e2e.execZarfCommand("init", "--components=gitops-service,logging", "--confirm")
	require.NoError(t, err, output)

	// Deploy the gitops example
	path := fmt.Sprintf("../../../build/zarf-package-gitops-service-data-%s.tar.zst", e2e.arch)
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
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
	client := &http.Client{Timeout: time.Second * 10}
	repoName := "mirror__repo1.dso.mil__platform-one__big-bang__apps__security-tools__twistlock"
	getRepoRequest, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/api/v1/repos/%v/%v", config.IPV4Localhost, k8s.PortGit, config.ZarfGitPushUser, repoName), nil)
	getRepoRequest.SetBasicAuth(config.ZarfGitReadUser, config.GetSecret(config.StateGitPull))
	getRepoRequest.Header.Add("accept", "application/json")
	getRepoRequest.Header.Add("Content-Type", "application/json")
	getRepoResponse, err := client.Do(getRepoRequest)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, 300, getRepoResponse.StatusCode)
	assert.LessOrEqual(t, 200, getRepoResponse.StatusCode)

	// Make sure the only permissions are pull (read)
	getRepoResponseBody, _ := io.ReadAll(getRepoResponse.Body)
	var bodyMap map[string]interface{}
	json.Unmarshal(getRepoResponseBody, &bodyMap)
	permissionsMap := bodyMap["permissions"].(map[string]interface{})
	assert.False(t, permissionsMap["admin"].(bool))
	assert.False(t, permissionsMap["push"].(bool))
	assert.True(t, permissionsMap["pull"].(bool))
}
