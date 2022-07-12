package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitAndFlux(t *testing.T) {
	t.Log("E2E: Git and flux")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-gitops-service-data-%s.tar.zst", e2e.arch)

	// Deploy the gitops example
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Create a tunnel to the git resources
	// Get a random local port for this instance
	localPort, _ := k8s.GetAvailablePort()
	gitUrl := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	testGitServerConnect(t, localPort, gitUrl)
	testGitServerReadOnly(t, gitUrl)
	waitFluxPodInfoDeployment(t)

	e2e.chartsToRemove = append(e2e.chartsToRemove,
		ChartTarget{
			namespace: "podinfo",
			name:      "zarf-raw-podinfo-via-flux",
		},
		ChartTarget{
			namespace: "flux",
			name:      "zarf-raw-flux-crds",
		},
		ChartTarget{
			namespace: "zarf",
			name:      "zarf-gitea",
		},
	)
}

func testGitServerConnect(t *testing.T, localPort int, gitUrl string) {
	// Establish the port-forward into the game service
	err := e2e.execZarfBackgroundCommand("connect", "git", fmt.Sprintf("--local-port=%d", localPort), "--cli-only", "-l=trace")
	require.NoError(t, err, "unable to connect to the git port-forward")

	// Make sure Gitea comes up cleanly
	resp, err := http.Get(gitUrl + "/explore/repos")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testGitServerReadOnly(t *testing.T, gitUrl string) {
	// Init the state variable
	state := k8s.LoadZarfState()
	config.InitState(state)

	// Get the repo as the readonly user
	repoName := "mirror__repo1.dso.mil__platform-one__big-bang__apps__security-tools__twistlock"
	getRepoRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s", gitUrl, config.ZarfGitPushUser, repoName), nil)
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

func waitFluxPodInfoDeployment(t *testing.T) {
	// Deploy the flux example and verify that it works
	path := fmt.Sprintf("build/zarf-package-flux-test-%s.tar.zst", e2e.arch)
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm", "-l=trace")
	require.NoError(t, err, stdOut, stdErr)

	var kubectlOut []byte
	timeout := time.After(3 * time.Minute)

timer:
	for {
		// delay check 3 seconds
		time.Sleep(2 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			t.Error("Timeout waiting for flux podinfo deployment")

			// after delay, try running
		default:
			// Check that flux deployed the podinfo example
			kubectlOut, err = exec.Command("kubectl", "wait", "deployment", "-n=podinfo", "podinfo", "--for", "condition=Available=True", "--timeout=3s").Output()
			// Log error
			if err != nil {
				t.Log(string(kubectlOut), err)
			} else {
				// Otherwise, break the loop and continue
				break timer
			}
		}
	}

	assert.Contains(t, string(kubectlOut), "condition met")
}
