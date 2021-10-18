package test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestTerraformSshExample(t *testing.T) {
	t.Parallel()

	// Copy the terraform folder to a temp directory so we can run multiple tests in parallel
	tmpFolder := teststructure.CopyTerraformFolderToTemp(t, "..", "tf/public-ec2-instance")

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(t, "TEARDOWN", func() {
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)
		aws.DeleteEC2KeyPair(t, keyPair)

		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		terraform.Destroy(t, terraformOptions)
	})

	// Deploy the terraform infra
	teststructure.RunTestStage(t, "SETUP", func() {
		terraformOptions, keyPair, err := configureTerraformOptions(t, tmpFolder)
		require.NoError(t, err)

		// Save the options and key pair so later test stages can use them
		teststructure.SaveTerraformOptions(t, tmpFolder, terraformOptions)
		teststructure.SaveEc2KeyPair(t, tmpFolder, keyPair)

		// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
		terraform.InitAndApply(t, terraformOptions)
	})

	// Upload the Zarf artifacts
	teststructure.RunTestStage(t, "UPLOAD", func() {
		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

		// This will upload the Zarf binary, init package, and other necessary files to the server so we can use them for
		// tests
		syncFilesToRemoteServer(t, terraformOptions, keyPair)
	})

	// Make sure we can SSH to the public Instance directly from the public Internet
	teststructure.RunTestStage(t, "TEST", func() {
		terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
		keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

		// Finally run the actual test
		test(t, terraformOptions, keyPair)
	})
}

func configureTerraformOptions(t *testing.T, tmpFolder string) (*terraform.Options, *aws.Ec2Keypair, error) {
	// A unique ID we can use to namespace resources so we don't clash with anything already in the AWS account or
	// tests running in parallel
	uniqueID := random.UniqueId()
	namespace := "zarf"
	stage := "terratest"
	name := fmt.Sprintf("e2e-%s", uniqueID)

	// Get the region to use from the system's environment
	awsRegion, err := getAwsRegion()
	if err != nil {
		return nil, nil, err
	}

	// Some AWS regions are missing certain instance types, so pick an available type based on the region we picked
	instanceType := aws.GetRecommendedInstanceType(t, awsRegion, []string{"t3a.large", "t3.large", "t2.large"})

	// Create an EC2 KeyPair that we can use for SSH access
	keyPairName := fmt.Sprintf("%s-%s-%s", namespace, stage, name)
	keyPair := aws.CreateAndImportEC2KeyPair(t, awsRegion, keyPairName)

	// Construct the terraform options with default retryable errors to handle the most common retryable errors in
	// terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: tmpFolder,

		// Variables to pass to our Terraform code using -var options
		Vars: map[string]interface{}{
			"aws_region":    awsRegion,
			"namespace":     namespace,
			"stage":         stage,
			"name":          name,
			"instance_type": instanceType,
			"key_pair_name": keyPairName,
		},
	})

	return terraformOptions, keyPair, nil
}

func syncFilesToRemoteServer(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair.KeyPair,
		SshUserName: "ubuntu",
	}

	// It can take a minute or so for the Instance to boot up, so retry a few times
	maxRetries := 15
	timeBetweenRetries, err := time.ParseDuration("5s")
	require.NoError(t, err)

	// Wait for the instance to be ready
	_, err = retry.DoWithRetryE(t, "Wait for the instance to be ready", maxRetries, timeBetweenRetries, func() (string, error) {
		_, err := ssh.CheckSshCommandE(t, publicHost, "whoami")
		if err != nil {
			return "", err
		}
		return "", nil
	})
	require.NoError(t, err)

	// Upload the compiled Zarf binary to the server. The ssh lib only supports sending strings so we'll base64encode it
	// first
	f, err := os.Open("../../build/zarf")
	require.NoError(t, err)
	reader := bufio.NewReader(f)
	content, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	encodedZarfBinary := base64.StdEncoding.EncodeToString(content)
	err = ssh.ScpFileToE(t, publicHost, 0644, "$HOME/zarf.b64", encodedZarfBinary)
	require.NoError(t, err)
	output, err := ssh.CheckSshCommandE(t, publicHost, "cd $HOME && sudo bash -c 'base64 -d zarf.b64 > /usr/local/bin/zarf && chmod 0777 /usr/local/bin/zarf'")
	require.NoError(t, err, output)

	// Upload zarf-init.tar.zst
	f, err = os.Open("../../build/zarf-init.tar.zst")
	require.NoError(t, err)
	reader = bufio.NewReader(f)
	content, err = ioutil.ReadAll(reader)
	require.NoError(t, err)
	encodedZarfInit := base64.StdEncoding.EncodeToString(content)
	err = ssh.ScpFileToE(t, publicHost, 0644, "$HOME/zarf-init.tar.zst.b64", encodedZarfInit)
	require.NoError(t, err)
	output, err = ssh.CheckSshCommandE(t, publicHost, "cd $HOME && base64 -d zarf-init.tar.zst.b64 > zarf-init.tar.zst")
	require.NoError(t, err, output)
}

func test(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	publicHost := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  keyPair.KeyPair,
		SshUserName: "ubuntu",
	}

	// Make sure `zarf --help` doesn't error
	output, err := ssh.CheckSshCommandE(t, publicHost, "zarf --help")
	require.NoError(t, err, output)

	// Test `zarf init just to make sure it returns a zero exit code.`
	output, err = ssh.CheckSshCommandE(t, publicHost, "sudo bash -c 'zarf init --confirm --components management,logging,gitops-service --host localhost'")
	require.NoError(t, err, output)
}

// getAwsRegion returns the desired AWS region to use by first checking the env var AWS_REGION, then checking
//AWS_DEFAULT_REGION if AWS_REGION isn't set. If neither is set it returns an error
func getAwsRegion() (string, error) {
	val, present := os.LookupEnv("AWS_REGION")
	if !present {
		val, present = os.LookupEnv("AWS_DEFAULT_REGION")
	}
	if !present {
		return "", fmt.Errorf("expected either AWS_REGION or AWS_DEFAULT_REGION env var to be set, but they were not")
	} else {
		return val, nil
	}
}
