package test

import (
	"fmt"
	"testing"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneralCli(t *testing.T) {

	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {
		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-package-kafka-strimzi-demo.tar.zst", fmt.Sprintf("/home/%s/build/zarf-package-kafka-strimzi-demo.tar.zst", e2e.username), "0700")
	})

	teststructure.RunTestStage(e2e.testing, "TEST", func() {
		// Test `zarf prepare sha256sum` for a local asset
		expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
		output, err := e2e.runSSHCommand("cd /home/%s/build && echo 'random test data 🦄' > shasum-test-file", e2e.username)
		require.NoError(e2e.testing, err, output)

		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf prepare sha256sum shasum-test-file 2> /dev/null", e2e.username)
		require.NoError(e2e.testing, err, output)
		assert.Equal(e2e.testing, expectedShasum, output, "The expected SHASUM should equal the actual SHASUM")

		// Test `zarf prepare sha256sum` for a remote asset
		expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"
		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf prepare sha256sum https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt 2> /dev/null", e2e.username)
		require.NoError(e2e.testing, err, output)
		assert.Equal(e2e.testing, expectedShasum, output, "The expected SHASUM should equal the actual SHASUM")

		// Test `zarf version`
		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf version", e2e.username)
		require.NoError(e2e.testing, err, output)
		assert.NotNil(e2e.testing, output)
		assert.NotEqual(e2e.testing, len(output), 0, "Zarf version should not be an empty string")
		assert.NotEqual(e2e.testing, string(output), "UnknownVersion", "Zarf version should not be the default value")

		// Test for expected failure when given a bad component input
		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf init --confirm --components k3s,foo,logging", e2e.username)
		require.Error(e2e.testing, err, output)

		// Test for expected failure when given invalid hostnames
		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf init --confirm --host localhost", e2e.username)
		require.Error(e2e.testing, err, output)

		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf pki regenerate --host zarf@server", e2e.username)
		require.Error(e2e.testing, err, output)
		output, err = e2e.runSSHCommand("cd /home/%s/build && ./zarf pki regenerate --host some_unique_server", e2e.username)
		require.Error(e2e.testing, err, output)

		// Initialize Zarf for the next set of tests
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Verify that we do not timeout when passing the `--confirm` flag without specifying the `--components` flag
		output, err = e2e.runSSHCommand("sudo timeout 120 sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-kafka-strimzi-demo.tar.zst --confirm' || false", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Test that `zarf package deploy` doesn't die when given a URL
		// NOTE: Temporarily commenting this out because this seems out of scope for a general cli test. Having this included also means we would have to fully standup a `zarf init` command.
		// TODO: Move this to it's own e2e test.
		// output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("sudo bash -c 'cd /home/%s/build && ./zarf package deploy https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom.tar.zst --confirm --insecure'", username))
		// require.NoError(t, err, output)
		// output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("sudo bash -c 'cd /home/%s/build && ./zarf package deploy https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom.tar.zst --confirm --shasum e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'", username))
		// require.NoError(t, err, output)

		// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf package deploy https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst --confirm'", e2e.username)
		require.Error(e2e.testing, err, output)

		// Test that changing the log level actually applies the requested level
		output, _ = e2e.runSSHCommand("cd /home/%s/build && ./zarf version --log-level warn 1> /dev/null", e2e.username)
		expectedOutString := "Log level set to warn"
		require.Contains(e2e.testing, output, expectedOutString, "The log level should be changed to 'warn'")
	})

}
