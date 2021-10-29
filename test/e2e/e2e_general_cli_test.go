package test

import (
	"fmt"
	"testing"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneralCli(t *testing.T) {
	t.Parallel()

	// Our SSH username, will change based on which AMI we use
	username := "ubuntu"

	// Copy the terraform folder to a temp directory so we can run multiple tests in parallel
	tmpFolder := teststructure.CopyTerraformFolderToTemp(t, "..", "tf/public-ec2-instance")

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(t, "TEARDOWN", func() {
		teardown(t, tmpFolder)
	})

	// Deploy the terraform infra
	teststructure.RunTestStage(t, "SETUP", func() {
		setup(t, tmpFolder)
	})

	// Upload the Zarf artifacts
	teststructure.RunTestStage(t, "UPLOAD", func() {
		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

		syncFileToRemoteServer(t, terraformOptions, keyPair, username, "../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", username), "0700")
	})

	teststructure.RunTestStage(t, "TEST", func() {
		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

		// Finally run the actual test
		testGeneralCliStuff(t, terraformOptions, keyPair, username)
	})
}

func testGeneralCliStuff(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair, username string) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair.KeyPair,
		SshUserName: username,
	}

	// Test `zarf prepare sha256sum` for a local asset
	expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"
	output, err := ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && echo 'random test data 🦄' > shasum-test-file", username))
	require.NoError(t, err, output)
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf prepare sha256sum shasum-test-file 2> /dev/null", username))
	require.NoError(t, err, output)
	assert.Equal(t, expectedShasum, output, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf prepare sha256sum` for a remote asset
	expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf prepare sha256sum https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt 2> /dev/null", username))
	require.NoError(t, err, output)
	assert.Equal(t, expectedShasum, output, "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf version`
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf version", username))
	require.NoError(t, err, output)
	assert.NotNil(t, output)
	assert.NotEqual(t, len(output), 0, "Zarf version should not be an empty string")
	assert.NotEqual(t, string(output), "UnknownVersion", "Zarf version should not be the default value")

	// Test for expected failure when given a bad component input
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf init --components management,foo,logging", username))
	require.Error(t, err, output)

	// Test for expected failure when given invalid hostnames
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf init --host bad!hostname", username))
	require.Error(t, err, output)
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf pki regenerate --host zarf@server", username))
	require.Error(t, err, output)
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("cd /home/%s/build && ./zarf pki regenerate --host some_unique_server", username))
	require.Error(t, err, output)

	// Test that `zarf package deploy` doesn't die when given a URL
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("sudo bash -c 'cd /home/%s/build && ./zarf package deploy https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom.tar.zst --confirm'", username))
}
