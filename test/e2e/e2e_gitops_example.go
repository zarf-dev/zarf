package test

import (
	"fmt"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGitopsExample(t *testing.T) {
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
		syncFileToRemoteServer(t, terraformOptions, keyPair, username, "../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", username), "0600")
		syncFileToRemoteServer(t, terraformOptions, keyPair, username, "../../build/zarf-package-gitops-service-data.tar.zst", fmt.Sprintf("/home/%s/build/zarf-package-gitops-service-data.tar.zst", username), "0600")
	})

	teststructure.RunTestStage(t, "TEST", func() {
		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

		// Finally run the actual test
		testGitopsExample(t, terraformOptions, keyPair, username)
	})
}

func testGitopsExample(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair, username string) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair.KeyPair,
		SshUserName: username,
	}

	// run `zarf init`
	output, err := ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components management,logging,gitops-service --host 127.0.0.1'", username))
	require.NoError(t, err, output)

	// Make sure Gitea comes up cleanly
	output, err = ssh.CheckSshCommandE(t, publicHost, "timeout 300 bash -c 'while [[ \"$(curl -sfSL --retry 15 --retry-connrefused --retry-delay 5 -o /dev/null -w \"%{http_code}\" \"https://127.0.0.1/api/v1/user\")\" != \"401\" ]]; do sleep 1; done' || false")
	require.NoError(t, err, output)

	// Deploy the gitops example
	output, err = ssh.CheckSshCommandE(t, publicHost, fmt.Sprintf("sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-gitops-service-data.tar.zst --confirm'", username))
	require.NoError(t, err, output)
}
