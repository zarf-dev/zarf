package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	test "github.com/defenseunicorns/zarf/src/test/e2e"
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
	os.Chdir("../../../../build/")

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
	// Initialize the cluster with the Git server and AMD64 architecture
	zarfExec(t, "init", "--confirm", "--components", "git-server", "--architecture", "amd64")

	// Build the previous version
	bbVersion := fmt.Sprintf("--set=BB_VERSION=%s", previous)
	zarfExec(t, "package", "create", "../src/extensions/bigbang/test/package", bbVersion, "--confirm")

	// Deploy the previous version
	pkgPath := fmt.Sprintf("zarf-package-big-bang-test-amd64-%s.tar.zst", previous)
	zarfExec(t, "package", "deploy", pkgPath, "--confirm")

	// Remove the previous version package
	_ = os.RemoveAll(pkgPath)

	// HACK: scale down the flux deployments due to very-low CPU in the test runner
	fluxControllers := []string{"helm-controller", "source-controller", "kustomize-controller", "notification-controller"}
	for _, deployment := range fluxControllers {
		zarfExec(t, "tools", "kubectl", "-n", "flux-system", "scale", "deployment", deployment, "--replicas=0")
	}

	// Cluster info
	zarfExec(t, "tools", "kubectl", "describe", "nodes")

	// Build the latest version
	bbVersion = fmt.Sprintf("--set=BB_VERSION=%s", latest)
	zarfExec(t, "package", "create", "../src/extensions/bigbang/test/package", bbVersion, "--confirm")

	// Clean up zarf cache now that all packages are built to reduce disk pressure
	zarfExec(t, "tools", "clear-cache")

	// Deploy the latest version
	pkgPath = fmt.Sprintf("zarf-package-big-bang-test-amd64-%s.tar.zst", latest)
	zarfExec(t, "package", "deploy", pkgPath, "--confirm")

	// Cluster info
	zarfExec(t, "tools", "kubectl", "describe", "nodes")

	// Test connectivity to Twistlock
	testConnection(t)
}

func testConnection(t *testing.T) {
	// Establish the tunnel config
	tunnel, err := cluster.NewTunnel("twistlock", "svc", "twistlock-console", 0, 8081)
	require.NoError(t, err)

	// Establish the tunnel connection
	tunnel.Connect("", false)
	defer tunnel.Close()

	// Test the connection
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func zarfExec(t *testing.T, args ...string) {
	err := exec.CmdWithPrint(zarf, args...)
	require.NoError(t, err)
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
