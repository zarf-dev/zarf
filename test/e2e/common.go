package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	k3dcluster "github.com/rancher/k3d/v5/cmd/cluster"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
)

type ZarfE2ETest struct {
	zarfBinPath string

	clusterName          string `default: "test-cluster"`
	kubeconfigPath       string
	filesToRemove        []string
	cmdsToKill           []*exec.Cmd
	kubeconfig           *os.File
	provider             *cluster.Provider
	restConfig           *restclient.Config
	clientset            *kubernetes.Clientset
	clusterAlreadyExists bool
}

func getKubeconfigPath() (string, error) {
	// k3dcluster.NewCmdClusterCreate()
	// Check if the $KUBECONFIG env is set
	// TODO: It would probably be good to verify a useable kubeconfig lives here
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		return kubeconfigEnv, nil
	}

	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configBaseDir := path.Join(userHomeDir, ".kube")
	if err := os.MkdirAll(configBaseDir, 0700); err != nil {
		return "", err
	}

	kubeconfigPath := path.Join(configBaseDir, "config")
	_, err = os.OpenFile(kubeconfigPath, os.O_RDWR|os.O_CREATE, 0755)
	return kubeconfigPath, err
}

func (e2e *ZarfE2ETest) setUpKind() error {

	// Determine what the name of the zarfBinary should be
	e2e.zarfBinPath = path.Join("../../build", getCLIName())

	var err error
	// Create or get the kubeconfig
	e2e.kubeconfigPath, err = getKubeconfigPath()
	if err != nil {
		return err
	}

	// Set up a KinD cluster if necessary
	e2e.provider = cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	nodes, err := e2e.provider.ListNodes(e2e.clusterName)
	if len(nodes) > 0 {
		// There already is a cluster up!! yay!!
		e2e.clusterAlreadyExists = true
	} else {
		err = e2e.provider.Create(
			e2e.clusterName,
			cluster.CreateWithNodeImage(""),
			cluster.CreateWithRetain(false),
			cluster.CreateWithWaitForReady(time.Duration(0)),
			cluster.CreateWithKubeconfigPath(e2e.kubeconfigPath),
			cluster.CreateWithDisplayUsage(false),
		)
	}

	// Get config and client for the k8s cluster
	e2e.restConfig, err = clientcmd.BuildConfigFromFlags("", e2e.kubeconfigPath)
	if err != nil {
		return err
	}
	e2e.clientset, err = kubernetes.NewForConfig(e2e.restConfig)
	if err != nil {
		return err
	}

	// Wait for the cluster to have pods before we let the test suite run
	attempt := 0
	for attempt < 10 {
		pods, err := e2e.clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
		if err == nil && len(pods.Items) >= 0 {
			fmt.Printf("💥 Cluster %s ready. You can access it by setting:\nexport KUBECONFIG='%s'\n", e2e.clusterName, e2e.kubeconfigPath)
			break
		}

		time.Sleep(1 * time.Second)
		attempt++
		if attempt > 15 {
			return errors.New("unable to connect to KinD cluster for e2e tests")
		}
	}

	return err
}

func (e2e *ZarfE2ETest) tearDownKind() error {
	if os.Getenv("SKIP_TEARDOWN") != "" || e2e.clusterAlreadyExists {
		return nil
	}

	// k3dcluster.NewCmdClusterCreate()

	// Delete the cluster and kubeconfig file
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Delete(e2e.clusterName, e2e.kubeconfigPath)
	os.Remove(e2e.kubeconfigPath)
	return err
}

func (e2e *ZarfE2ETest) setUpK3D() error {

	// Determine what the name of the zarfBinary should be
	e2e.zarfBinPath = path.Join("../../build", getCLIName())

	var err error
	// Create or get the kubeconfig
	e2e.kubeconfigPath, err = getKubeconfigPath()
	if err != nil {
		return err
	}

	createClusterCommand := k3dcluster.NewCmdClusterCreate()
	err = createClusterCommand.ExecuteContext(context.TODO())
	if err != nil {
		fmt.Println("ERROR WHEN TRYING TO SET UP K3D CLUSTER")
		return err
	}

	// Get config and client for the k8s cluster
	e2e.restConfig, err = clientcmd.BuildConfigFromFlags("", e2e.kubeconfigPath)
	if err != nil {
		return err
	}
	e2e.clientset, err = kubernetes.NewForConfig(e2e.restConfig)
	if err != nil {
		return err
	}

	// Wait for the cluster to have pods before we let the test suite run
	attempt := 0
	for attempt < 10 {
		pods, err := e2e.clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
		if err == nil && len(pods.Items) >= 0 {
			fmt.Printf("💥 Cluster %s ready. You can access it by setting:\nexport KUBECONFIG='%s'\n", e2e.clusterName, e2e.kubeconfigPath)
			break
		}

		time.Sleep(1 * time.Second)
		attempt++
		if attempt > 15 {
			return errors.New("unable to connect to KinD cluster for e2e tests")
		}
	}
	return nil
}

func (e2e *ZarfE2ETest) tearDownK3D() error {
	deleteClusterCommand := k3dcluster.NewCmdClusterDelete()
	err := deleteClusterCommand.ExecuteContext(context.TODO())
	return err
}

func getCLIName() string {
	var binaryName string
	if runtime.GOOS == "linux" {
		binaryName = "zarf"
	} else if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			binaryName = "zarf-mac-apple"
		} else {
			binaryName = "zarf-mac-intel"
		}
	}
	return binaryName
}

func (e2e *ZarfE2ETest) cleanupAfterTest(t *testing.T) {
	// Use Zarf to perform chart uninstallation
	output, err := e2e.execZarfCommand("destroy", "--confirm", "--remove-components", "-l=trace")
	require.NoError(t, err, output)

	// Remove files created for the test
	for _, filePath := range e2e.filesToRemove {
		err = os.RemoveAll(filePath)
		require.NoError(t, err, "unable to remove file when cleaning up after a test")
	}
	e2e.filesToRemove = []string{}

	// Kill background processes spawned during the test
	for _, cmd := range e2e.cmdsToKill {
		if cmd.Process != nil {
			err = cmd.Process.Kill()
			require.NoError(t, err, "unable to kill background cmd when cleaning up after a test")
		}
	}
	e2e.cmdsToKill = []*exec.Cmd{}
}

func (e2e *ZarfE2ETest) execCommandInPod(podname, namespace string, cmd []string) (string, string, error) {
	stdoutBuffer := &strings.Builder{}
	stderrBuffer := &strings.Builder{}

	req := e2e.clientset.CoreV1().RESTClient().Post().Resource("pods").Name(podname).Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	req.VersionedParams(option, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(e2e.restConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: stdoutBuffer,
		Stderr: stderrBuffer,
	})

	return stdoutBuffer.String(), stderrBuffer.String(), err
}

// TODO: It might be a nice feature to read some flag/env and change the stdout and stderr to pipe to the terminal running the test
func (e2e *ZarfE2ETest) execZarfCommand(commandString ...string) (string, error) {
	output, err := exec.Command(e2e.zarfBinPath, commandString...).CombinedOutput()
	return string(output), err
}

func (e2e *ZarfE2ETest) execZarfBackgroundCommand(commandString ...string) error {
	// Create a tunnel to the git resources
	tunnelCmd := exec.Command(e2e.zarfBinPath, commandString...)
	err := tunnelCmd.Start()
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	return err
}
