package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test"
	"github.com/stretchr/testify/require"
)

// Code related to fetching the last two Big Bang versions
// and using them to set the BB_VERSION and BB_MAJOR variables
// has been commented out due to a bug in how Zarf clones and checks out git repos.
//
// https://github.com/defenseunicorns/zarf/actions/runs/8529925302/job/23403205495?pr=2411#step:9:897
//
// The versions are currently hardcoded to the last two known working versions.
// TODO: fix the git clone/checkout bug and update this test to not be hardcoded.

// The Big Bang project ID on Repo1
// const bbProjID = "2872"

var (
	zarf string
	// previous string
	// latest string
)

func TestMain(m *testing.M) {
	// Change to the build dir
	if err := os.Chdir("../../../../build/"); err != nil {
		panic(err)
	}

	// // Get the latest and previous releases
	// latest, previous, err = getReleases()
	// if err != nil {
	// 	panic(err)
	// }

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

	// Initialize the cluster with the Git server and AMD64 architecture
	arch := "amd64"
	stdOut, stdErr, err := zarfExec("init", "--components", "git-server", "--architecture", arch, tmpdir, "--confirm", zarfCache)
	require.NoError(t, err, stdOut, stdErr)

	// Remove the init package to free up disk space on the test runner
	err = os.RemoveAll(fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, getZarfVersion(t)))
	require.NoError(t, err)

	// Build the previous version
	bbVersion := "--set=BB_VERSION=2.22.0"
	bbMajor := "--set=BB_MAJOR=2"
	stdOut, stdErr, err = zarfExec("package", "create", "../src/extensions/bigbang/test/package", bbVersion, bbMajor, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Clean up zarf cache to reduce disk pressure
	stdOut, stdErr, err = zarfExec("tools", "clear-cache")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the previous version
	pkgPath := fmt.Sprintf("zarf-package-big-bang-test-%s-2.22.0.tar.zst", arch)
	stdOut, stdErr, err = zarfExec("package", "deploy", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// HACK: scale down the flux deployments due to very-low CPU in the test runner
	fluxControllers := []string{"helm-controller", "source-controller", "kustomize-controller", "notification-controller"}
	for _, deployment := range fluxControllers {
		stdOut, stdErr, err = zarfExec("tools", "kubectl", "-n", "flux-system", "scale", "deployment", deployment, "--replicas=0")
		require.NoError(t, err, stdOut, stdErr)
	}

	// Cluster info
	stdOut, stdErr, err = zarfExec("tools", "kubectl", "describe", "nodes")
	require.NoError(t, err, stdOut, stdErr)

	// Build the latest version
	bbVersion = "--set=BB_VERSION=2.23.0"
	bbMajor = "--set=BB_MAJOR=2"
	stdOut, stdErr, err = zarfExec("package", "create", "../src/extensions/bigbang/test/package", bbVersion, bbMajor, "--differential", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the previous version package
	err = os.RemoveAll(pkgPath)
	require.NoError(t, err)

	// Clean up zarf cache to reduce disk pressure
	stdOut, stdErr, err = zarfExec("tools", "clear-cache")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the latest version
	pkgPath = fmt.Sprintf("zarf-package-big-bang-test-%s-2.22.0-differential-2.23.0.tar.zst", arch)
	stdOut, stdErr, err = zarfExec("package", "deploy", pkgPath, tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Cluster info
	stdOut, stdErr, err = zarfExec("tools", "kubectl", "describe", "nodes")
	require.NoError(t, err, stdOut, stdErr)

	// Test connectivity to Twistlock
	testConnection(t)
}

func testConnection(t *testing.T) {
	// Establish the tunnel config
	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnel, err := c.NewTunnel("twistlock", "svc", "twistlock-console", "", 0, 8081)
	require.NoError(t, err)

	// Establish the tunnel connection
	_, err = tunnel.Connect()
	require.NoError(t, err)
	defer tunnel.Close()

	// Test the connection
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func zarfExec(args ...string) (string, string, error) {
	return exec.CmdWithContext(context.TODO(), exec.PrintCfg(), zarf, args...)
}

// getZarfVersion returns the current build/zarf version
func getZarfVersion(t *testing.T) string {
	// Get the version of the CLI
	stdOut, stdErr, err := zarfExec("version")
	require.NoError(t, err, stdOut, stdErr)
	return strings.Trim(stdOut, "\n")
}

// func getReleases() (latest, previous string, err error) {
// 	// Create the URL for the API endpoint
// 	url := fmt.Sprintf("https://repo1.dso.mil/api/v4/projects/%s/repository/tags", bbProjID)

// 	// Send an HTTP GET request to the API endpoint
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		return latest, previous, err
// 	}
// 	defer resp.Body.Close()

// 	// Read the response body
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return latest, previous, err
// 	}

// 	// Parse the response body as a JSON array of objects
// 	var data []map[string]interface{}
// 	err = json.Unmarshal(body, &data)
// 	if err != nil {
// 		return latest, previous, err
// 	}

// 	// Compile the regular expression for filtering tags that don't contain a hyphen
// 	re := regexp.MustCompile("^[^-]+$")

// 	// Create a slice to store the tag names that match the regular expression
// 	var releases []string

// 	// Iterate over the tags returned by the API, and filter out tags that don't match the regular expression
// 	for _, tag := range data {
// 		name := tag["name"].(string)
// 		if re.MatchString(name) {
// 			releases = append(releases, name)
// 		}
// 	}

// 	// Set the latest and previous release variables to the first two releases
// 	return releases[0], releases[1], nil
// }
