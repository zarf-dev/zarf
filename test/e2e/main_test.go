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
	err := e2e.setUpKind()
	if err != nil {
		fmt.Printf("Unable to setup environment to run the e2e test because of err: %v\n", err)
		os.Exit(1)
	}

	// This defer teardown still runs if a panic/fatal happens while running the tests
	defer e2e.tearDownKind()

	// exec test and this returns an exit code to pass to os
	retCode := m.Run()

	// Teardown the cluster now that tests are completed
	e2e.tearDownKind()

	// time.Sleep(15 * time.Second)

	// err = e2e.setUpK3D()
	// if err != nil {
	// 	fmt.Printf("unable to set up k3d environment to run the e2e tests on")
	// }

	// retCode = m.Run()
	// e2e.tearDownK3D()

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}
