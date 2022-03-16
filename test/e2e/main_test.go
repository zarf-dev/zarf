package test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/defenseunicorns/zarf/cli/config"
)

var (
	e2e ZarfE2ETest
)

// TestMain will exec each test, one by one
func TestMain(m *testing.M) {
	retCode := 0

	e2e.arch = config.GetArch()

	// Set up constants for the tests
	e2e.zarfBinPath = path.Join("../../build", getCLIName())

	// Run the tests if a valid cluster exists or we're being told to create the K3s cluster
	if e2e.checkIfClusterRunning() || shouldCreateK3sCluster() {
		retCode = m.Run()
	} else {
		fmt.Printf(`Unable to run tests. No valid cluster present and TESTDISTRO != "k3s"`)
		retCode = 1
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}
