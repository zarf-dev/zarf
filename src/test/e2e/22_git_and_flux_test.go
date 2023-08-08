// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/stretchr/testify/require"
)

func TestGitAndFlux(t *testing.T) {
	t.Log("E2E: Git and flux")
	e2e.SetupWithCluster(t)
	tmpdir := t.TempDir()
	cachePath := filepath.Join(tmpdir, ".cache-location")

	buildPath := filepath.Join("src", "test", "packages", "22-git-and-flux")
	stdOut, stdErr, err := e2e.Zarf("package", "create", buildPath, "--zarf-cache", cachePath, "-o=build", "--confirm", "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)

	path := fmt.Sprintf("build/zarf-package-git-data-test-%s-1.0.0.tar.zst", e2e.Arch)
	defer e2e.CleanFiles(path)

	// Deploy the gitops example
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// This package contains SBOMable things but was created with --skip-sbom
	require.Contains(t, string(stdErr), "This package does NOT contain an SBOM.")

	tunnel, err := cluster.NewZarfTunnel()
	require.NoError(t, err)
	err = tunnel.Connect(cluster.ZarfGit, false)
	require.NoError(t, err)
	defer tunnel.Close()

	testGitServerConnect(t, tunnel.HTTPEndpoint())
	testGitServerReadOnly(t, tunnel.HTTPEndpoint())
	testGitServerTagAndHash(t, tunnel.HTTPEndpoint())
	waitFluxPodInfoDeployment(t)
}

func testGitServerConnect(t *testing.T, gitURL string) {
	// Make sure Gitea comes up cleanly
	resp, err := http.Get(gitURL + "/explore/repos")
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func testGitServerReadOnly(t *testing.T, gitURL string) {
	// Init the state variable
	state, err := cluster.NewClusterOrDie().LoadZarfState()
	require.NoError(t, err)

	gitCfg := git.New(state.GitServer)

	// Get the repo as the readonly user
	repoName := "zarf-public-test-2469062884"
	getRepoRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s", gitURL, state.GitServer.PushUsername, repoName), nil)
	getRepoResponseBody, err := gitCfg.DoHTTPThings(getRepoRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	require.NoError(t, err)

	// Make sure the only permissions are pull (read)
	var bodyMap map[string]interface{}
	json.Unmarshal(getRepoResponseBody, &bodyMap)
	permissionsMap := bodyMap["permissions"].(map[string]interface{})
	require.False(t, permissionsMap["admin"].(bool))
	require.False(t, permissionsMap["push"].(bool))
	require.True(t, permissionsMap["pull"].(bool))
}

func testGitServerTagAndHash(t *testing.T, gitURL string) {
	// Init the state variable
	state, err := cluster.NewClusterOrDie().LoadZarfState()
	require.NoError(t, err, "Failed to load Zarf state")
	repoName := "zarf-public-test-2469062884"

	gitCfg := git.New(state.GitServer)

	// Get the Zarf repo tag
	repoTag := "v0.0.1"
	getRepoTagsRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s/tags/%s", gitURL, config.ZarfGitPushUser, repoName, repoTag), nil)
	getRepoTagsResponseBody, err := gitCfg.DoHTTPThings(getRepoTagsRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	require.NoError(t, err)

	// Make sure the pushed tag exists
	var tagMap map[string]interface{}
	json.Unmarshal(getRepoTagsResponseBody, &tagMap)
	require.Equal(t, repoTag, tagMap["name"])

	// Get the Zarf repo commit
	repoHash := "01a23218923f24194133b5eb11268cf8d73ff1bb"
	getRepoCommitsRequest, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s/git/commits/%s", gitURL, config.ZarfGitPushUser, repoName, repoHash), nil)
	getRepoCommitsResponseBody, err := gitCfg.DoHTTPThings(getRepoCommitsRequest, config.ZarfGitReadUser, state.GitServer.PullPassword)
	require.NoError(t, err)
	require.Contains(t, string(getRepoCommitsResponseBody), repoHash)
}

func waitFluxPodInfoDeployment(t *testing.T) {
	// Deploy the flux example and verify that it works
	path := fmt.Sprintf("build/zarf-package-podinfo-flux-%s.tar.zst", e2e.Arch)
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the flux example when deployment completes
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "podinfo-flux", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
