package test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	if os.Getenv("SKIP_TEARDOWN") == "" && !e2e.clusterAlreadyExists {
		err = e2e.tearDown()
		if err != nil {
			fmt.Printf("Unable to teardown test environment after completion of tests: %v\n", err)
		}
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}

func TestGeneralCLI(t *testing.T) {
	// Test `zarf prepare sha256sum` for a local asset
	expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"

	// TODO: There has to be a better way to pipe this output to the file.. For some reason exec.Command( ... > file ).Output() couldn't pipe to file
	// output, err = exec.Command("bash", "-c", "\"echo 'random test data 🦄' > shasum-test-file\"").Output()
	shasumTestFilePath := "shasum-test-file"
	testfile, _ := os.Create(shasumTestFilePath)
	cmd := exec.Command("echo", "random test data 🦄")
	cmd.Stdout = testfile
	cmd.Run()

	output, err := exec.Command(e2e.zarfBinPath, "prepare", "sha256sum", shasumTestFilePath).Output()
	assert.NoError(t, err, output)
	assert.Equal(t, expectedShasum, string(output), "The expected SHASUM should equal the actual SHASUM")
	os.Remove(shasumTestFilePath)

	// Test `zarf prepare sha256sum` for a remote asset
	expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"
	output, err = exec.Command(e2e.zarfBinPath, "prepare", "sha256sum", "https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt").Output()
	assert.NoError(t, err, output)
	assert.Equal(t, expectedShasum, string(output), "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf version`
	output, err = exec.Command(e2e.zarfBinPath, "version").Output()
	assert.NoError(t, err)
	assert.NotEqual(t, len(output), 0, "Zarf version should not be an empty string")
	assert.NotEqual(t, string(output), "UnknownVersion", "Zarf version should not be the default value")

	// Test for expected failure when given a bad componenet input
	output, err = exec.Command(e2e.zarfBinPath, "init", "--confirm", "--components=k3s,foo,logging").Output()
	assert.Error(t, err)

	// Test for expected failure when given invalid hostnames
	output, err = exec.Command(e2e.zarfBinPath, "pki", "regenerate", "--host", "zarf@server").Output()
	assert.Error(t, err, output)
	output, err = exec.Command(e2e.zarfBinPath, "pki", "regenerate", "--host=some_unique_server").Output()
	assert.Error(t, err, output)

	// Test that changing the log level actually applies the requested level
	output, _ = exec.Command(e2e.zarfBinPath, "version", "--log-level=warn").CombinedOutput()
	expectedOutString := "Log level set to warn"
	require.Contains(t, string(output), expectedOutString, "The log level should be changed to 'warn'")
}

func TestInitZarf(t *testing.T) {

	// Initialize Zarf for the next set of tests
	// This also confirms that using the `--confirm` flags does not hang when not also specifying the `--components` flag
	output, err := exec.Command(e2e.zarfBinPath, "init", "--confirm").CombinedOutput()
	assert.NoError(t, err, string(output))

	// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
	output, err = exec.Command(e2e.zarfBinPath, "package", "deploy", "https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst", "--confirm").Output()
	assert.Error(t, err, string(output))
}
