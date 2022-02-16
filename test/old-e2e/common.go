package test

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/retry"
	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
)

type ZarfE2ETest struct {
	testing          *testing.T
	tempFolder       string
	username         string
	terraformOptions *terraform.Options
	keyPair          *aws.Ec2Keypair
	publicIP         string
	publicHost       ssh.Host
}

func NewE2ETest(testing *testing.T) *ZarfE2ETest {

	testing.Parallel()

	// Copy the terraform folder to a temp directory so we can run multiple tests in parallel
	tempFolder := teststructure.CopyTerraformFolderToTemp(testing, "..", "tf/public-ec2-instance")

	e2e := ZarfE2ETest{
		testing:    testing,
		tempFolder: tempFolder,
		// Our SSH username, will change based on which AMI we use
		username: "ubuntu",
	}

	// Deploy the terraform infra
	teststructure.RunTestStage(testing, "SETUP", e2e.setup)

	return &e2e
}

func (e2e *ZarfE2ETest) runSSHCommand(format string, a ...interface{}) (string, error) {
	command := fmt.Sprintf(format, a...)
	return ssh.CheckSshCommandE(e2e.testing, e2e.publicHost, command)
}

func (e2e *ZarfE2ETest) teardown() {
	keyPair := teststructure.LoadEc2KeyPair(e2e.testing, e2e.tempFolder)
	aws.DeleteEC2KeyPair(e2e.testing, keyPair)

	terraformOptions := teststructure.LoadTerraformOptions(e2e.testing, e2e.tempFolder)
	terraform.Destroy(e2e.testing, terraformOptions)
}

func (e2e *ZarfE2ETest) setup() {
	terraformOptions, keyPair, err := e2e.configureTerraformOptions()
	require.NoError(e2e.testing, err)

	// Save the options and key pair so later test stages can use them
	teststructure.SaveTerraformOptions(e2e.testing, e2e.tempFolder, terraformOptions)
	teststructure.SaveEc2KeyPair(e2e.testing, e2e.tempFolder, keyPair)

	// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
	terraform.InitAndApply(e2e.testing, terraformOptions)

	// Run `terraform output` to get the value of an output variable
	e2e.publicIP = terraform.Output(e2e.testing, terraformOptions, "public_instance_ip")
	e2e.terraformOptions = terraformOptions
	e2e.keyPair = keyPair

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	e2e.publicHost = ssh.Host{
		Hostname:    e2e.publicIP,
		SshKeyPair:  e2e.keyPair.KeyPair,
		SshUserName: e2e.username,
	}
}

func (e2e *ZarfE2ETest) configureTerraformOptions() (*terraform.Options, *aws.Ec2Keypair, error) {
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

	instanceType := "t3a.large"

	// Create an EC2 KeyPair that we can use for SSH access
	keyPairName := fmt.Sprintf("%s-%s-%s", namespace, stage, name)
	keyPair := aws.CreateAndImportEC2KeyPair(e2e.testing, awsRegion, keyPairName)

	// Construct the terraform options with default retryable errors to handle the most common retryable errors in
	// terraform testing.
	terraformOptions := terraform.WithDefaultRetryableErrors(e2e.testing, &terraform.Options{
		// The path to where our Terraform code is located
		TerraformDir: e2e.tempFolder,

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

// syncFileToRemoteServer uses SCP to sync a file from source to destination. `destPath` can be absolute or relative to
// the SSH user's home directory. It has to be in a directory that the SSH user is allowed to write to.
func (e2e *ZarfE2ETest) syncFileToRemoteServer(srcPath string, destPath string, chmod string) {
	// Run `terraform output` to get the value of an output variable
	publicInstanceIP := terraform.Output(e2e.testing, e2e.terraformOptions, "public_instance_ip")

	// We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
	// as we know the Instance is running an Ubuntu AMI that has such a user
	host := ssh.Host{
		Hostname:    publicInstanceIP,
		SshKeyPair:  e2e.keyPair.KeyPair,
		SshUserName: e2e.username,
	}

	// It can take a minute or so for the Instance to boot up, so retry a few times
	maxRetries := 15
	timeBetweenRetries, err := time.ParseDuration("5s")
	require.NoError(e2e.testing, err)

	// Wait for the instance to be ready
	_, err = retry.DoWithRetryE(e2e.testing, "Wait for the instance to be ready", maxRetries, timeBetweenRetries, func() (string, error) {
		_, err := ssh.CheckSshCommandE(e2e.testing, host, "whoami")
		if err != nil {
			return "", err
		}
		return "", nil
	})
	require.NoError(e2e.testing, err)

	// Create the folder structure
	output, err := ssh.CheckSshCommandE(e2e.testing, host, fmt.Sprintf("bash -c 'install -m 644 -D /dev/null \"%s\"'", destPath))
	require.NoError(e2e.testing, err, output)

	// The ssh lib only supports sending strings so we'll base64encode it first
	f, err := os.Open(srcPath)
	require.NoError(e2e.testing, err)
	reader := bufio.NewReader(f)
	content, err := ioutil.ReadAll(reader)
	require.NoError(e2e.testing, err)
	encodedContent := base64.StdEncoding.EncodeToString(content)
	err = ssh.ScpFileToE(e2e.testing, host, 0600, fmt.Sprintf("%s.b64", destPath), encodedContent)
	require.NoError(e2e.testing, err)
	output, err = ssh.CheckSshCommandE(e2e.testing, host, fmt.Sprintf("base64 -d \"%s.b64\" > \"%s\" && chmod \"%s\" \"%s\"", destPath, destPath, chmod, destPath))
	require.NoError(e2e.testing, err, output)
}

// getAwsRegion returns the desired AWS region to use by first checking the env var AWS_REGION, then checking
// AWS_DEFAULT_REGION if AWS_REGION isn't set. If neither is set it returns an error
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
