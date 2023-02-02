// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitAndFlux(t *testing.T) {
	t.Log("E2E: Git and flux")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// path := fmt.Sprintf("build/zarf-package-git-data-%s-v1.0.0.tar.zst", e2e.arch)

	// // Deploy the gitops example
	// stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	// require.NoError(t, err, stdOut, stdErr)

	// tunnel, err := cluster.NewZarfTunnel()
	// require.NoError(t, err)
	// tunnel.Connect(cluster.ZarfGit, false)
	// defer tunnel.Close()

	// testGitServerConnect(t, tunnel.HTTPEndpoint())
	// testGitServerReadOnly(t, tunnel.HTTPEndpoint())
	// testGitServerTagAndHash(t, tunnel.HTTPEndpoint())
	// waitFluxPodInfoDeployment(t)

	// stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "flux-test", "--confirm")
	// require.NoError(t, err, stdOut, stdErr)

	// stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "init", "--components=git-server", "--confirm")
	// require.NoError(t, err, stdOut, stdErr)

	testRemovingTagsOnCreate(t)
}

func testGitServerConnect(t *testing.T, gitURL string) {
	// Make sure Gitea comes up cleanly
	resp, err := http.Get(gitURL + "/explore/repos")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testGitServerReadOnly(t *testing.T, gitURL string) {
	// Init the state variable
	state, err := cluster.NewClusterOrDie().LoadZarfState()
	require.NoError(t, err)

	gitCfg := git.New(state.GitServer)

	// Get the repo as the readonly user
	repoName := "zarf-1211668992"
	getRepoRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s", gitURL, state.GitServer.PushUsername, repoName), nil)
	getRepoResponseBody, err := gitCfg.DoHTTPThings(getRepoRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	assert.NoError(t, err)

	// Make sure the only permissions are pull (read)
	var bodyMap map[string]interface{}
	json.Unmarshal(getRepoResponseBody, &bodyMap)
	permissionsMap := bodyMap["permissions"].(map[string]interface{})
	assert.False(t, permissionsMap["admin"].(bool))
	assert.False(t, permissionsMap["push"].(bool))
	assert.True(t, permissionsMap["pull"].(bool))
}

func testGitServerTagAndHash(t *testing.T, gitURL string) {
	// Init the state variable
	state, err := cluster.NewClusterOrDie().LoadZarfState()
	require.NoError(t, err, "Failed to load Zarf state")
	repoName := "zarf-1211668992"

	gitCfg := git.New(state.GitServer)

	// Get the Zarf repo tag
	repoTag := "v0.15.0"
	getRepoTagsRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s/tags/%s", gitURL, config.ZarfGitPushUser, repoName, repoTag), nil)
	getRepoTagsResponseBody, err := gitCfg.DoHTTPThings(getRepoTagsRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	assert.NoError(t, err)

	// Make sure the pushed tag exists
	var tagMap map[string]interface{}
	json.Unmarshal(getRepoTagsResponseBody, &tagMap)
	assert.Equal(t, repoTag, tagMap["name"])

	// Get the Zarf repo commit
	repoHash := "c74e2e9626da0400e0a41e78319b3054c53a5d4e"
	getRepoCommitsRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s/commits", gitURL, config.ZarfGitPushUser, repoName), nil)
	getRepoCommitsResponseBody, err := gitCfg.DoHTTPThings(getRepoCommitsRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	assert.NoError(t, err)

	// Make sure the pushed commit exists
	var commitMap []map[string]interface{}
	json.Unmarshal(getRepoCommitsResponseBody, &commitMap)
	assert.Equal(t, repoHash, commitMap[0]["sha"])
}

func waitFluxPodInfoDeployment(t *testing.T) {
	// Deploy the flux example and verify that it works
	path := fmt.Sprintf("build/zarf-package-flux-test-%s.tar.zst", e2e.arch)
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
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

			break timer

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

func testRemovingTagsOnCreate(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	require.NoError(t, err, "unable to get the user's home directory")

	// Build the test package
	testPackageDirPath := "src/test/test-packages/git-repo-caching"
	testPackagePath := fmt.Sprintf("%s/zarf-package-test-package-git-cache-%s.tar.zst", testPackageDirPath, e2e.arch)
	outputFlag := fmt.Sprintf("-o=%s", testPackageDirPath)
	_, _, err = e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm")
	require.NoError(t, err, "error when building the test package")
	defer e2e.cleanFiles(testPackagePath)

	// Extract the built package so we can inspect the repositories that are included
	extractedDirPath := "tmp-extraction"
	stdOut, stdErr, err := e2e.execZarfCommand("tools", "archiver", "decompress", testPackagePath, extractedDirPath, "-l=trace")
	defer e2e.cleanFiles(extractedDirPath)
	require.NoError(t, err, stdOut, stdErr)

	/* Test to make sure we are removing tags where necessary */
	// verify the component has multiple tags (in this case, we want to make sure it includes something we didn't specifically ask for)
	gitDirFlag := fmt.Sprintf("--git-dir=%s/components/full-repo/repos/zarf-1211668992/.git", extractedDirPath)
	gitTagOut, err := exec.Command("git", gitDirFlag, "tag", "-l").Output()
	require.NoError(t, err)
	require.Contains(t, string(gitTagOut), "v0.22.0")

	// verify the component has only a single tag
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-tag/repos/zarf-1211668992/.git", extractedDirPath)
	gitTagOut, err = exec.Command("git", gitDirFlag, "tag", "-l").Output()
	require.NoError(t, err)
	require.Equal(t, "v0.16.0\n", string(gitTagOut))

	/* Test to make sure we are pulling the latest upstream changes when building a package */
	cachedRepoGitDirFlag := fmt.Sprintf("--git-dir=%s/.zarf-cache/repos/zarf-1211668992/.git", homeDir)
	cachedRepoWorkTreeFlag := fmt.Sprintf("--work-tree=%s/.zarf-cache/repos/zarf-1211668992", homeDir)
	_, err = exec.Command("git", cachedRepoGitDirFlag, cachedRepoWorkTreeFlag, "reset", "HEAD~3", "--hard").Output()
	require.NoError(t, err, err)

	// Make sure the cache is now 'old'
	statusOutput, err := exec.Command("git", cachedRepoGitDirFlag, "status").Output()
	require.NoError(t, err, err)
	require.Contains(t, string(statusOutput), "Your branch is behind")

	// Re-build the test package
	_, _, err = e2e.execZarfCommand("package", "create", testPackageDirPath, outputFlag, "--confirm")
	require.NoError(t, err, "error when rebuilding the test git-cache package")

	// Check to make sure the cache is no longer 'old'
	statusOutput, err = exec.Command("git", cachedRepoGitDirFlag, cachedRepoWorkTreeFlag, "status").Output()
	require.NoError(t, err, err)
	require.Contains(t, string(statusOutput), "Your branch is up to date")
}
