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

func teardown(t *testing.T, tmpFolder string) {
  keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)
  aws.DeleteEC2KeyPair(t, keyPair)

  terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
  terraform.Destroy(t, terraformOptions)
}

func setup(t *testing.T, tmpFolder string) {
  terraformOptions, keyPair, err := configureTerraformOptions(t, tmpFolder)
  require.NoError(t, err)

  // Save the options and key pair so later test stages can use them
  teststructure.SaveTerraformOptions(t, tmpFolder, terraformOptions)
  teststructure.SaveEc2KeyPair(t, tmpFolder, keyPair)

  // This will run `terraform init` and `terraform apply` and fail the test if there are any errors
  terraform.InitAndApply(t, terraformOptions)
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

  instanceType := "t3a.large"

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

// syncFileToRemoteServer uses SCP to sync a file from source to destination. `destPath` can be absolute or relative to
// the SSH user's home directory. It has to be in a directory that the SSH user is allowed to write to.
func syncFileToRemoteServer(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair, sshUsername string, srcPath string, destPath string, chmod string) {
  // Run `terraform output` to get the value of an output variable
  publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

  // We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
  // as we know the Instance is running an Ubuntu AMI that has such a user
  host := ssh.Host{
    Hostname:    publicInstanceIP,
    SshKeyPair:  keyPair.KeyPair,
    SshUserName: sshUsername,
  }

  // It can take a minute or so for the Instance to boot up, so retry a few times
  maxRetries := 15
  timeBetweenRetries, err := time.ParseDuration("5s")
  require.NoError(t, err)

  // Wait for the instance to be ready
  _, err = retry.DoWithRetryE(t, "Wait for the instance to be ready", maxRetries, timeBetweenRetries, func() (string, error){
    _, err := ssh.CheckSshCommandE(t, host, "whoami")
    if err != nil {
      return "", err
    }
    return "", nil
  })
  require.NoError(t, err)

  // Create the folder structure
  output, err := ssh.CheckSshCommandE(t, host,fmt.Sprintf("bash -c 'install -m 644 -D /dev/null \"%s\"'", destPath))
  require.NoError(t, err, output)

  // The ssh lib only supports sending strings so we'll base64encode it first
  f, err := os.Open(srcPath)
  require.NoError(t, err)
  reader := bufio.NewReader(f)
  content, err := ioutil.ReadAll(reader)
  require.NoError(t, err)
  encodedContent := base64.StdEncoding.EncodeToString(content)
  err = ssh.ScpFileToE(t, host, 0600, fmt.Sprintf("%s.b64", destPath), encodedContent)
  require.NoError(t, err)
  output, err = ssh.CheckSshCommandE(t, host, fmt.Sprintf("base64 -d \"%s.b64\" > \"%s\" && chmod \"%s\" \"%s\"", destPath, destPath, chmod, destPath))
  require.NoError(t, err, output)
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
