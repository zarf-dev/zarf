package test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/test/e2e/clusters"
)

var (
	e2e ZarfE2ETest //nolint:gochecknoglobals
)

const (
	testDistroEnvVarName = "TESTDISTRO"
)

// TestMain lets us customize the test run. See https://medium.com/goingogo/why-use-testmain-for-testing-in-go-dafb52b406bc
func TestMain(m *testing.M) {
	// Work from the root directory of the project
	os.Chdir("../../../")

	// K3d use the intern package, which requires this to be set in go 1.18
	os.Setenv("ASSUME_NO_MOVING_GC_UNSAFE_RISK_IT_WITH", "go1.18")

	retCode, err := doAllTheThings(m)
	if err != nil {
		fmt.Println(err) //nolint:forbidigo
	}
	os.Exit(retCode)
}

// doAllTheThings just wraps what should go in TestMain. It's in its own function so it can
// [a] Not have a bunch of `os.Exit()` calls in it
// [b] Do defers properly
// [c] Handle errors cleanly
//
// It returns the return code passed from `m.Run()` and any error thrown.
func doAllTheThings(m *testing.M) (int, error) {
	var err error

	// Set up constants in the global variable that all the tests are able to access
	e2e.arch = config.GetArch()

	e2e.zarfBinPath = path.Join("build", getCLIName())
	e2e.distroToUse, err = clusters.GetDistroToUseFromString(os.Getenv(testDistroEnvVarName))
	if err != nil {
		fmt.Println("Env var TESTDISTRO (provided|kind|k3d|k3s) not specified, using k3d")
		e2e.distroToUse = clusters.DistroK3d
	}

	// Validate that the Zarf binary exists. If it doesn't that means the dev hasn't built it, usually by running
	// `make build-cli`
	_, err = os.Stat(e2e.zarfBinPath)
	if err != nil {
		return 1, fmt.Errorf("zarf binary %v not found", e2e.zarfBinPath)
	}

	// If needed:
	// [1] Create the cluster with a temporary kubeconfig file
	// [2] Update the the KUBECONFIG env var
	// [3] Defer the cluster destroy and deletion of the temp file. We don't need to set the env var back since it was
	//     only changed for the current process.
	if e2e.distroToUse == clusters.DistroKind || e2e.distroToUse == clusters.DistroK3d {
		// Create the cluster
		tempKubeconfigFilePath, err := clusters.CreateClusterWithTemporaryKubeconfig(e2e.distroToUse)
		if err != nil {
			return 1, fmt.Errorf("unable to create %v cluster: %w", os.Getenv(testDistroEnvVarName), err)
		}

		// Defer cleanup
		defer func(tempKubeconfigFilePath string) {
			err := clusters.DeleteClusterAndTemporaryKubeconfig(e2e.distroToUse, tempKubeconfigFilePath)
			if err != nil {
				fmt.Println(fmt.Errorf("unable to delete cluster and temporary kubeconfig: %w", err)) //nolint:forbidigo
			}

			e2e.cleanupAfterAllTests()
		}(tempKubeconfigFilePath)
	}

	// Run the tests, with the cluster cleanup being deferred to the end of the function call
	retcode := m.Run()

	return retcode, nil
}
