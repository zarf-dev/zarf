package test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/test/e2e/clusters"
)

var (
	e2e ZarfE2ETest //nolint:gochecknoglobals
)

const (
	testDistroEnvVarName = "TESTDISTRO"
)

// TestMain lets us customize the test run. See https://medium.com/goingogo/why-use-testmain-for-testing-in-go-dafb52b406bc
func TestMain(m *testing.M) {
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

	e2e.zarfBinPath = path.Join("../../build", getCLIName())
	e2e.distroToUse, err = clusters.GetDistroToUseFromString(os.Getenv(testDistroEnvVarName))
	if err != nil {
		return 1, fmt.Errorf("unable to determine which k8s cluster to use. Env var TESTDISTRO must be present with "+
			"value [provided|kind|k3d|k3s]: %v", err)
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
		}(tempKubeconfigFilePath)
	}

	// Final check to make sure we have a working k8s cluster, skipped if we are using K3s
	if e2e.distroToUse != clusters.DistroK3s {
		err = clusters.TryValidateClusterIsRunning()
		if err != nil {
			return 1, fmt.Errorf("unable to connect to a running k8s cluster: %w", err)
		}
	}

	// Run the tests, with the cluster cleanup being deferred to the end of the function call
	retcode := m.Run()

	return retcode, nil
}
