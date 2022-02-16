package test

import (
	"fmt"
	"os"
	"testing"
)

var (
	e2e ZarfE2ETest
)

// TestMain will exec each test, one by one
func TestMain(m *testing.M) {
	// Create a kubeconfig and start up a KinD cluster
	err := e2e.setUp()
	if err != nil {
		fmt.Printf("Unable to setup environment to run the e2e test because of err: %v\n", err)
		os.Exit(1)
	}

	// exec test and this returns an exit code to pass to os
	retCode := m.Run()

	// Unless told to skip, destroy the KinD Cluster and delete the generated kubeconfig
	// TODO: Should add some defer logic here so that this gets executed even if there is a fatal error in the tests
	if os.Getenv("SKIP_TEARDOWN") == "" && !e2e.clusterAlreadyExists {
		err = e2e.tearDown()
		if err != nil {
			fmt.Printf("Unable to teardown test environment after completion of tests: %v\n", err)
		}
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}
