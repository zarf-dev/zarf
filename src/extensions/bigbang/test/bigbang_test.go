// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

// The Big Bang project ID on Repo1
const bbProjID = "2872"

var (
	zarf     string
	previous string
	latest   string
)

func TestMain(m *testing.M) {
	var err error

	// Change to the build dir
	if err := os.Chdir("../../../../build/"); err != nil {
		panic(err)
	}

	// Get the latest and previous releases
	latest, previous, err = getReleases()
	if err != nil {
		panic(err)
	}

	// Get the Zarf CLI path
	zarf = fmt.Sprintf("./%s", test.GetCLIName())

	// Run the tests
	m.Run()
}

func TestReleases(t *testing.T) {
	CIMount := "/mnt/zarf-tmp"
	tmpdir := fmt.Sprintf("--tmpdir=%s", t.TempDir())
	zarfCache := ""
	// If we are in CI set the temporary directory to /mnt/zarf-tmp to reduce disk pressure
	if os.Getenv("CI") == "true" {
		tmpdir = fmt.Sprintf("--tmpdir=%s", CIMount)
		zarfCache = fmt.Sprintf("--zarf-cache=%s", CIMount)
	}

	ctx := context.Background()

	// Initialize the cluster with the Git server and AMD64 architecture
	arch := "amd64"
	stdOut, stdErr, err := zarfExec(ctx, "init", "--components", "git-server", "--architecture", arch, tmpdir, "--confirm", zarfCache)
	require.NoError(t, err, stdOut, stdErr)

	// Remove the init package to free up disk space on the test runner
	err = os.RemoveAll(fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, getZarfVersion(ctx, t)))
	require.NoError(t, err)

	// Build the previous version
	bbVersion := fmt.Sprintf("--set=BB_VERSION=%s", previous)
	bbMajor := fmt.Sprintf("--set=BB_MAJOR=%s", previous[0:1])
	stdOut, stdErr, err = zarfExec(ctx, "package", "create", "../src/extensions/bigbang/test/package", bbVersion, bbMajor, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Clean up zarf cache to reduce disk pressure
	stdOut, stdErr, err = zarfExec(ctx, "tools", "clear-cache")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the previous version
	pkgPath := fmt.Sprintf("zarf-package-big-bang-test-%s-%s.tar.zst", arch, previous)
	stdOut, stdErr, err = zarfExec(ctx, "package", "deploy", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// HACK: scale down the flux deployments due to very-low CPU in the test runner
	fluxControllers := []string{"helm-controller", "source-controller", "kustomize-controller", "notification-controller"}
	for _, deployment := range fluxControllers {
		stdOut, stdErr, err = zarfExec(ctx, "tools", "kubectl", "-n", "flux-system", "scale", "deployment", deployment, "--replicas=0")
		require.NoError(t, err, stdOut, stdErr)
	}

	// Cluster info
	stdOut, stdErr, err = zarfExec(ctx, "tools", "kubectl", "describe", "nodes")
	require.NoError(t, err, stdOut, stdErr)

	// Build the latest version
	bbVersion = fmt.Sprintf("--set=BB_VERSION=%s", latest)
	bbMajor = fmt.Sprintf("--set=BB_MAJOR=%s", latest[0:1])
	stdOut, stdErr, err = zarfExec(ctx, "package", "create", "../src/extensions/bigbang/test/package", bbVersion, bbMajor, "--differential", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the previous version package
	err = os.RemoveAll(pkgPath)
	require.NoError(t, err)

	// Clean up zarf cache to reduce disk pressure
	stdOut, stdErr, err = zarfExec(ctx, "tools", "clear-cache")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the latest version
	pkgPath = fmt.Sprintf("zarf-package-big-bang-test-%s-%s-differential-%s.tar.zst", arch, previous, latest)
	stdOut, stdErr, err = zarfExec(ctx, "package", "deploy", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Cluster info
	stdOut, stdErr, err = zarfExec(ctx, "tools", "kubectl", "describe", "nodes")
	require.NoError(t, err, stdOut, stdErr)

	// Test connectivity to Twistlock
	testConnection(ctx, t)
}

func testConnection(ctx context.Context, t *testing.T) {
	// Establish the tunnel config
	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnel := c.CreateTunnel("twistlock", "svc", "twistlock-console", "", 0, 8081)

	// Establish the tunnel connection
	_, err = tunnel.Connect(ctx)
	require.NoError(t, err)
	defer tunnel.Close()

	// Test the connection
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func zarfExec(ctx context.Context, args ...string) (string, string, error) {
	return exec.CmdWithContext(ctx, exec.PrintCfg(), zarf, args...)
}

// getZarfVersion returns the current build/zarf version
func getZarfVersion(ctx context.Context, t *testing.T) string {
	// Get the version of the CLI
	stdOut, stdErr, err := zarfExec(ctx, "version")
	require.NoError(t, err, stdOut, stdErr)
	return strings.Trim(stdOut, "\n")
}

func getReleases() (latest, previous string, err error) {
	// Create the URL for the API endpoint
	url := fmt.Sprintf("https://repo1.dso.mil/api/v4/projects/%s/repository/tags", bbProjID)

	// Send an HTTP GET request to the API endpoint
	resp, err := http.Get(url)
	if err != nil {
		return latest, previous, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return latest, previous, err
	}

	// Parse the response body as a JSON array of objects
	var data []map[string]interface{}
	err = json.Unmarshal(body, &data)
	if err != nil {
		return latest, previous, err
	}

	// Compile the regular expression for filtering tags that don't contain a hyphen
	re := regexp.MustCompile("^[^-]+$")

	// Create a slice to store the tag names that match the regular expression
	var releases []string

	// Iterate over the tags returned by the API, and filter out tags that don't match the regular expression
	for _, tag := range data {
		name := tag["name"].(string)
		if re.MatchString(name) {
			releases = append(releases, name)
		}
	}

	// Set the latest and previous release variables to the first two releases
	return releases[0], releases[1], nil
}
