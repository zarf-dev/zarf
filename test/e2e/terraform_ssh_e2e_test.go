package test

import (
  "bufio"
  "encoding/base64"
  "fmt"
  "github.com/gruntwork-io/terratest/modules/logger"
  "github.com/gruntwork-io/terratest/modules/random"
  "github.com/stretchr/testify/require"
  "io/ioutil"
  "os"
  "testing"
  "time"

  "github.com/gruntwork-io/terratest/modules/aws"
  "github.com/gruntwork-io/terratest/modules/retry"
  "github.com/gruntwork-io/terratest/modules/ssh"
  "github.com/gruntwork-io/terratest/modules/terraform"
  teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestTerraformSshExample(t *testing.T) {
  t.Parallel()

  // Copy the terraform folder to a temp directory so we can run multiple tests in parallel
  tmpFolder := teststructure.CopyTerraformFolderToTemp(t, "..", "tf/public-ec2-instance")

  // At the end of the test, run `terraform destroy` to clean up any resources that were created
  defer teststructure.RunTestStage(t, "teardown", func() {
    keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)
    aws.DeleteEC2KeyPair(t, keyPair)

    terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
    terraform.Destroy(t, terraformOptions)
  })

  // Deploy the terraform infra
  teststructure.RunTestStage(t, "setup", func() {
    terraformOptions, keyPair, err := configureTerraformOptions(t, tmpFolder)
    require.NoError(t, err)

    // Save the options and key pair so later test stages can use them
    teststructure.SaveTerraformOptions(t, tmpFolder, terraformOptions)
    teststructure.SaveEc2KeyPair(t, tmpFolder, keyPair)

    // This will run `terraform init` and `terraform apply` and fail the test if there are any errors
    terraform.InitAndApply(t, terraformOptions)
  })

  // Make sure we can SSH to the public Instance directly from the public Internet
  teststructure.RunTestStage(t, "validate", func() {
    terraformOptions := teststructure.LoadTerraformOptions(t, tmpFolder)
    keyPair := teststructure.LoadEc2KeyPair(t, tmpFolder)

    uploadFilesToPublicHost(t, terraformOptions, keyPair)
    testZarfE2EOnPublicHost(t, terraformOptions, keyPair)
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
  instanceType := aws.GetRecommendedInstanceType(t, awsRegion, []string{"t3.medium", "t2.medium"})

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

func uploadFilesToPublicHost(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair) {
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
  maxRetries := 30
  timeBetweenRetries := 5 * time.Second
  description := fmt.Sprintf("SSH to public host %s", publicInstanceIP)

  // Upload the compiled Zarf binary to the server. The ssh lib only supports sending strings so we'll base64encode it
  // first
  f, err := os.Open("../../build/zarf")
  require.NoError(t, err)
  reader := bufio.NewReader(f)
  content, err := ioutil.ReadAll(reader)
  require.NoError(t, err)
  encodedZarfBinary := base64.StdEncoding.EncodeToString(content)
  retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
    err := ssh.ScpFileToE(t, publicHost, 0644, "$HOME/zarf.b64", encodedZarfBinary)
    if err != nil {
      return "", err
    }
    return "", nil
  })
  output, err := ssh.CheckSshCommandE(t, publicHost, "sudo bash -c 'base64 -d $HOME/zarf.b64 > /usr/local/bin/zarf && chmod 0777 /usr/local/bin/zarf'")
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
  output, err = ssh.CheckSshCommandE(t, publicHost, "base64 -d $HOME/zarf-init.tar.zst.b64 > $HOME/zarf-init.tar.zst")
  require.NoError(t, err, output)

  // Upload scripts/e2e.sh
  e2eSshScriptContents, err := ioutil.ReadFile("../scripts/e2e.sh")
  require.NoError(t, err)
  err = ssh.ScpFileToE(t, publicHost, 0777, "$HOME/e2e.sh", string(e2eSshScriptContents))
  require.NoError(t, err)
  output, err = ssh.CheckSshCommandE(t, publicHost, "sudo bash -c 'cp $HOME/e2e.sh /usr/local/bin/e2e.sh'")
  require.NoError(t, err, output)
}

func testZarfE2EOnPublicHost(t *testing.T, terraformOptions *terraform.Options, keyPair *aws.Ec2Keypair) {
  // Run `terraform output` to get the value of an output variable
  publicInstanceIP := terraform.Output(t, terraformOptions, "public_instance_ip")

  // We're going to try to SSH to the instance IP, using the Key Pair we created earlier, and the user "ubuntu",
  // as we know the Instance is running an Ubuntu AMI that has such a user
  publicHost := ssh.Host{
    Hostname:    publicInstanceIP,
    SshKeyPair:  keyPair.KeyPair,
    SshUserName: "ubuntu",
  }

  // It can take some time for Docker to be ready from the instance userdata
  maxRetries := 30
  timeBetweenRetries := 5 * time.Second
  description := fmt.Sprintf("SSH to public host %s", publicInstanceIP)

  // Make sure Docker works before proceeding
  retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
    // Make sure `docker info` works
    output, err := ssh.CheckSshCommandE(t, publicHost, "docker info")
    if err != nil {
      logger.Default.Logf(t, output)
      return "", err
    }

    // Make sure `docker run --rm hello-world` works
    output, err = ssh.CheckSshCommandE(t, publicHost, "docker run --rm hello-world")
    if err != nil {
      logger.Default.Logf(t, output)
      return "", err
    }
    return "", nil
  })

  // Make sure `zarf --help` doesn't error
  output, err := ssh.CheckSshCommandE(t, publicHost, "zarf --help")
  require.NoError(t, err, output)

  // Get the username and password for registry1.dso.mil. We'll need it in a bit
  username, password, err := getRegistry1Creds()
  require.NoError(t, err)

  // Log into registry1.dso.mil - Need to do it in a hacky way so it doesn't log secrets to stdout
  err = ssh.ScpFileToE(t, publicHost, 0600, "$HOME/registry1creds.env", fmt.Sprintf( "export REGISTRY1_USERNAME=%s; export REGISTRY1_PASSWORD=%s", username, password))
  require.NoError(t, err)
  output, err = ssh.CheckSshCommandE(t, publicHost, "source $HOME/registry1creds.env && zarf tools registry login registry1.dso.mil --username $REGISTRY1_USERNAME --password $REGISTRY1_PASSWORD")
  require.NoError(t, err, output)

  // Make sure e2e.sh runs and doesn't error
  output, err = ssh.CheckSshCommandE(t, publicHost, "sudo e2e.sh")
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

// getRegistry1Creds returns the username and password from environment variables, or an error if they aren't found
func getRegistry1Creds() (string, string, error) {
  usernameEnvVarName := "REGISTRY1_USERNAME"
  passwordEnvVarName := "REGISTRY1_PASSWORD"
  username, present := os.LookupEnv(usernameEnvVarName)
  if !present {
    return "", "", fmt.Errorf("expected env var %s not found", usernameEnvVarName)
  }
  password, present := os.LookupEnv(passwordEnvVarName)
  if !present {
    return "", "", fmt.Errorf("expected env var %s not found", passwordEnvVarName)
  }
  return username, password, nil
}
