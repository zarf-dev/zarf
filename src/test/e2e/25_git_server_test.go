package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitServer(t *testing.T) {
	t.Log("E2E: Git server")
	e2e.setup(t)
	defer e2e.teardown(t)

	repoPodInfo := "mirror__github.com__stefanprodan__podinfo"
	repoZarf := "mirror__github.com__defenseunicorns__zarf"
	gitUser := config.ZarfGitPushUser

	e2e.cleanFiles(repoPodInfo)
	e2e.cleanFiles(repoZarf)

	path := fmt.Sprintf("build/zarf-package-gitops-service-data-%s.tar.zst", e2e.arch)

	// Deploy the gitops example
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Create a tunnel to the git resources
	// Get a random local port for this instance
	localPort, _ := k8s.GetAvailablePort()

	// Establish the port-forward into the game service
	err = e2e.execZarfBackgroundCommand("connect", "git", fmt.Sprintf("--local-port=%d", localPort), "--cli-only")
	require.NoError(t, err, "unable to connect to the git port-forward")

	// Check for full git repo mirror (foo.git) from https://github.com/stefanprodan/podinfo.git
	adminPassword, _, err := e2e.execZarfCommand("tools", "get-admin-password")
	assert.NoError(t, err, "Unable to get admin password for gitea instance")
	pwdText := strings.TrimSpace(string(adminPassword))

	gitUrl := fmt.Sprintf("http://127.0.0.1:%d", localPort)
	gitAuthUrl := fmt.Sprintf("http://%s:%s@127.0.0.1:%d", gitUser, pwdText, localPort)

	clone := fmt.Sprintf("%s/%s/%s.git", gitAuthUrl, gitUser, repoPodInfo)
	gitOutput, err := exec.Command("git", "clone", clone).CombinedOutput()
	assert.NoError(t, err, string(gitOutput))

	// Check for tagged git repo mirror (foo.git@1.2.3) from https://github.com/defenseunicorns/zarf.git@v0.15.0
	clone = fmt.Sprintf("%s/%s/%s.git", gitAuthUrl, gitUser, repoZarf)
	gitOutput, err = exec.Command("git", "clone", clone).CombinedOutput()
	assert.NoError(t, err, string(gitOutput))

	// Check for correct tag
	expectedTag := "v0.15.0\n"
	assert.NoError(t, err)
	gitOutput, _ = exec.Command("git", "-C="+repoZarf, "tag").Output()
	assert.Equal(t, expectedTag, string(gitOutput), "Expected tag should match output")

	// Check for correct commits
	expectedCommits := "9eb207e\n7636dd0\ne02cec9"
	gitOutput, err = exec.Command("git", "log", "-3", "--oneline", "--pretty=format:%h").CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	assert.Equal(t, expectedCommits, string(gitOutput), "Expected commits should match output")

	// Check for existence of tags without specifying them, signifying that not using '@1.2.3' syntax brought over the whole repo
	expectedTag = "0.2.2"
	gitOutput, err = exec.Command("git", "-C="+repoPodInfo, "tag").CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	assert.Contains(t, string(gitOutput), expectedTag)

	// Make sure Gitea comes up cleanly
	resp, err := http.Get(gitUrl + "/explore/repos")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Init the state variable
	state := k8s.LoadZarfState()
	config.InitState(state)

	// Get the repo as the readonly user
	repoName := "mirror__repo1.dso.mil__platform-one__big-bang__apps__security-tools__twistlock"
	getRepoRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s", gitUrl, gitUser, repoName), nil)
	getRepoResponseBody, err := git.DoHttpThings(getRepoRequest, config.ZarfGitReadUser, config.GetSecret(config.StateGitPull))
	assert.NoError(t, err)

	// Make sure the only permissions are pull (read)
	var bodyMap map[string]interface{}
	json.Unmarshal(getRepoResponseBody, &bodyMap)
	permissionsMap := bodyMap["permissions"].(map[string]interface{})
	assert.False(t, permissionsMap["admin"].(bool))
	assert.False(t, permissionsMap["push"].(bool))
	assert.True(t, permissionsMap["pull"].(bool))

	e2e.cleanFiles(repoPodInfo)
	e2e.cleanFiles(repoZarf)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-gitea",
	})
}
