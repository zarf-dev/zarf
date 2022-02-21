package test

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

type testSuite struct {
	setupFunction   func() error
	cleanupFunction func() error
}

var (
	e2e ZarfE2ETest

	distroTests = map[string]testSuite{
		"k3d": {
			setupFunction:   e2e.setUpK3D,
			cleanupFunction: e2e.tearDownK3D,
		},
		"kind": {
			setupFunction:   e2e.setUpKind,
			cleanupFunction: e2e.tearDownKind,
		},
		"k3s": {
			setupFunction:   e2e.setUpK3s,
			cleanupFunction: e2e.tearDownK3s,
		},
	}
)

// TestMain will exec each test, one by one
func TestMain(m *testing.M) {
	retCode := 0

	distroToUse := strings.Split(os.Getenv("TESTDISTRO"), ",")
	if len(distroToUse) == 1 && distroToUse[0] == "" {
		// Use all the distros
		for key := range distroTests {
			distroToUse = append(distroToUse, key)
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
		defer testSuiteFunctions.cleanupFunction()
		if err != nil {
			fmt.Printf("Unable to setup %s environment to run the e2e test because of err: %v\n", distroName, err)
			os.Exit(1)
		}

		// exec test and capture exit code to pass to os
		testCode := m.Run()
		retCode = testCode | retCode

		// Teardown the cluster now that tests are completed
		err = testSuiteFunctions.cleanupFunction()
		if err != nil {
			fmt.Printf("Unable to cleanly teardown %s environment because of err: %v\n", distroName, err)
		}
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}
