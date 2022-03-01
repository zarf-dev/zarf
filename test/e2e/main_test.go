package test

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
)

type testSuite struct {
	standard         bool
	setupFunction    func() error
	tearDownFunction func() error
}

var (
	e2e         ZarfE2ETest
	static      string
	distroTests = map[string]testSuite{
		"k3d": {
			standard:         true,
			setupFunction:    e2e.setUpK3D,
			tearDownFunction: e2e.tearDownK3D,
		},
		"kind": {
			standard:         true,
			setupFunction:    e2e.setUpKind,
			tearDownFunction: e2e.tearDownKind,
		},
		"k3s": {
			standard:         false,
			setupFunction:    e2e.setUpK3s,
			tearDownFunction: e2e.tearDownK3s,
		},
	}
)

// TestMain will exec each test, one by one
func TestMain(m *testing.M) {
	var err error
	retCode := 0

	// Set up constants for the tests
	e2e.zarfBinPath = path.Join("../../build", getCLIName())
	e2e.kubeconfigPath, err = getKubeconfigPath()
	if err != nil {
		fmt.Printf("Unable to get the kubeconfig path for running the e2e tests because of err :%v\n", err)
		os.Exit(1)
	}

	// Check if a valid kubeconfig exists
	validCubeConfig := e2e.checkIfClusterRunning()

	// If the current kubeconfig points to an existing cluster run the tests against that cluster
	if validCubeConfig {
		retCode = m.Run()
		os.Exit(retCode)
	}

	// Run the tests against all the distros provided
	distroToUse := strings.Split(os.Getenv("TESTDISTRO"), ",")
	if len(distroToUse) == 1 && distroToUse[0] == "" {
		distroToUse = []string{}
		// No distros were specified; Use all the standard distros
		for key, value := range distroTests {
			if value.standard {
				distroToUse = append(distroToUse, key)
			}
		}
	}

	for _, distroName := range distroToUse {
		testSuiteFunctions, exists := distroTests[distroName]
		if !exists {
			fmt.Printf("Provided distro %v is not recognized, continuing tests but reporting as failure\n", distroName)
			retCode = 1
			continue
		}

		// Setup the cluster
		err := testSuiteFunctions.setupFunction()
		defer testSuiteFunctions.tearDownFunction()
		if err != nil {
			fmt.Printf("Unable to setup %s environment to run the e2e test because of err: %v\n", distroName, err)
			os.Exit(1)
		}

		// exec test and capture exit code to pass to os
		testCode := m.Run()
		retCode = testCode | retCode

		// Teardown the cluster now that tests are completed
		err = testSuiteFunctions.tearDownFunction()
		if err != nil {
			fmt.Printf("Unable to cleanly teardown %s environment because of err: %v\n", distroName, err)
		}
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}
