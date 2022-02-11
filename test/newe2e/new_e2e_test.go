package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

var (
	clusterName    string = "test-cluster"
	kubeconfigPath string
	kubeconfig     *os.File
	provider       *cluster.Provider
	restConfig     *restclient.Config
	clientset      *kubernetes.Clientset
)

// TestMain will exec each test, one by one
func TestMain(m *testing.M) {
	// Create a kubeconfig and start up a KinD cluster
	err := setUp()
	if err != nil {
		fmt.Printf("Unable to setup environment to run the e2e test because of err: %v\n", err)
		os.Exit(1)
	}

	// exec test and this returns an exit code to pass to os
	retCode := m.Run()

	// Destroy the KinD Cluster and delete the generated kubeconfig
	err = tearDown()
	if err != nil {
		fmt.Printf("Unable to teardown test environment after completion of tests: %v\n", err)
	}

	// If exit code is distinct of zero, the test will be failed (red)
	os.Exit(retCode)
}

func setUp() error {
	// Create the kubeconfig
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// configBaseDir := path.Join(userHomeDir, ".kube", "e2econfig")
	configBaseDir := path.Join(userHomeDir, ".kube")
	if err := os.MkdirAll(configBaseDir, 0700); err != nil {
		return err
	}

	// kubeconfigPath = path.Join(configBaseDir, clusterName)
	kubeconfigPath = path.Join(configBaseDir, "config")
	_, err = os.OpenFile(kubeconfigPath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}

	provider = cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	nodes, err := provider.ListNodes(clusterName)
	if len(nodes) > 0 {
		// There already is a cluster up!! yay!!

	} else {
		err = provider.Create(
			clusterName,
			cluster.CreateWithNodeImage(""),
			cluster.CreateWithRetain(false),
			cluster.CreateWithWaitForReady(time.Duration(0)),
			cluster.CreateWithKubeconfigPath(kubeconfigPath),
			cluster.CreateWithDisplayUsage(false),
			// cluster.CreateWithRawConfig([]byte(kindConfig)),
		)
	}

	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	time.Sleep(3 * time.Second)
	fmt.Printf("💥 Cluster %s ready. You can access it by setting:\nexport KUBECONFIG='%s'\n", clusterName, kubeconfigPath)
	return err
}

func tearDown() error {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Delete(clusterName, kubeconfigPath)
	os.Remove(kubeconfigPath)

	return err
}

func TestKindDeploy(t *testing.T) {
	// provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	pods, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})

	assert.NoError(t, err)
	assert.Greater(t, len(pods.Items), 0)
}

func TestGeneralCLI(t *testing.T) {
	// Test `zarf prepare sha256sum` for a local asset
	expectedShasum := "61b50898f982d015ed87093ba822de0fe011cec6dd67db39f99d8c56391a6109\n"

	// TODO: @JPERRY There has to be a better way to pipe this output to the file.. Why didn't the exec.Command() work again?
	// output, err = exec.Command("bash", "-c", "\"echo 'random test data 🦄' > shasum-test-file\"").Output()
	testfile, _ := os.Create("shasum-test-file")
	cmd := exec.Command("echo", "random test data 🦄")
	cmd.Stdout = testfile
	cmd.Start()
	cmd.Wait()

	output, err := exec.Command("../../build/zarf-mac-intel", "prepare", "sha256sum", "shasum-test-file").Output()
	assert.NoError(t, err, output)
	assert.Equal(t, expectedShasum, string(output), "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf prepare sha256sum` for a remote asset
	expectedShasum = "c3cdea0573ba5a058ec090b5d2683bf398e8b1614c37ec81136ed03b78167617\n"
	output, err = exec.Command("../../build/zarf-mac-intel", "prepare", "sha256sum", "https://zarf-public.s3-us-gov-west-1.amazonaws.com/pipelines/zarf-prepare-shasum-remote-test-file.txt").Output()
	assert.NoError(t, err, output)
	assert.Equal(t, expectedShasum, string(output), "The expected SHASUM should equal the actual SHASUM")

	// Test `zarf version`
	output, err = exec.Command("../../build/zarf-mac-intel", "version").Output()
	assert.NoError(t, err)
	assert.NotEqual(t, len(output), 0, "Zarf version should not be an empty string")
	assert.NotEqual(t, string(output), "UnknownVersion", "Zarf version should not be the default value")

	// Test for expected failure when given a bad componenet input
	output, err = exec.Command("../../build/zarf-mac-intel", "init", "--confirm", "--components=k3s,foo,logging").Output()
	assert.Error(t, err)

	// Test for expected failure when given invalid hostnames
	// NOTE: This next test doesn't work anymore because there is a kubeconfig that exists so the code skips the --host flag..
	// output, err = exec.Command("../../build/zarf-mac-intel", "init", "--confirm", "--host", "localhost").Output()
	// assert.Error(t, err, output)
	output, err = exec.Command("../../build/zarf-mac-intel", "pki", "regenerate", "--host", "zarf@server").Output()
	assert.Error(t, err, output)
	output, err = exec.Command("../../build/zarf-mac-intel", "pki", "regenerate", "--host=some_unique_server").Output()
	assert.Error(t, err, output)

	// Test that changing the log level actually applies the requested level
	output, _ = exec.Command("../../build/zarf-mac-intel", "version", "--log-level=warn").CombinedOutput()
	expectedOutString := "Log level set to warn"
	require.Contains(t, string(output), expectedOutString, "The log level should be changed to 'warn'")
}

func TestInitZarf(t *testing.T) {

	// Bleh.. need to figure out a better way to do this differently
	output, err := exec.Command("mv", kubeconfigPath, "/Users/jon/.kube/config").Output()
	fmt.Printf("output of the mv command: %v\n", string(output))
	fmt.Printf("should have moved the kubeconfig to %v\n", kubeconfigPath)
	// Initialize Zarf for the next set of tests
	// This also confirms that using the `--confirm` flags does not hang when not also specifying the `--components` flag
	output, err = exec.Command("../../build/zarf-mac-intel", "init", "--confirm").CombinedOutput()
	assert.NoError(t, err, string(output))

	// Test that `zarf package deploy` gives an error if deploying a remote package without the --insecure or --shasum flags
	output, err = exec.Command("../../build/zarf-mac-intel", "package", "deploy", "https://zarf-examples.s3.amazonaws.com/zarf-package-appliance-demo-doom-20210125.tar.zst", "--confirm").Output()
	assert.Error(t, err, string(output))

}
