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

	clusterName          string
	kubeconfigPath       string
	filesToRemove        []string
	cmdsToKill           []*exec.Cmd
	provider             *cluster.Provider
	restConfig           *restclient.Config
	clientset            *kubernetes.Clientset
	clusterAlreadyExists bool
	initWithK3s          bool
}

func getKubeconfigPath() (string, error) {
	// Check if the $KUBECONFIG env is set
	kubeconfigEnv := os.Getenv("KUBECONFIG")
	if kubeconfigEnv != "" {
		return kubeconfigEnv, nil
	}

	// Get the kubeconfig in ~/.kube/config
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configBaseDir := path.Join(userHomeDir, ".kube")
	if err := os.MkdirAll(configBaseDir, 0700); err != nil {
		return "", err
	}

	// Get (or create) the config file
	kubeconfigPath := path.Join(configBaseDir, "config")
	_, err = os.OpenFile(kubeconfigPath, os.O_RDWR|os.O_CREATE, 0755)
	return kubeconfigPath, err
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

func (e2e *ZarfE2ETest) buildConfigAndClientset() error {
	var err error
	e2e.restConfig, err = clientcmd.BuildConfigFromFlags("", e2e.kubeconfigPath)
	if err != nil {
		return err
	}
	e2e.clientset, err = kubernetes.NewForConfig(e2e.restConfig)

	return err
}

func (e2e *ZarfE2ETest) setUpKind() error {
	var err error

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
		if err != nil {
			return err
		}
	}

	// Get config and client for the k8s cluster
	err = e2e.buildConfigAndClientset()
	if err != nil {
		return err
	}

	// Wait for the cluster to have pods before we let the test suite run
	err = e2e.waitForHealthyCluster()

	return err
}

func (e2e *ZarfE2ETest) tearDownKind() error {
	if os.Getenv("SKIP_TEARDOWN") != "" || e2e.clusterAlreadyExists {
		return nil
	}

	// Delete the cluster and kubeconfig file
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	err := provider.Delete(e2e.clusterName, e2e.kubeconfigPath)
	os.Remove(e2e.kubeconfigPath)
	return err
}

func (e2e *ZarfE2ETest) setUpK3D() error {
	var err error

	createClusterCommand := k3dcluster.NewCmdClusterCreate()
	err = createClusterCommand.ExecuteContext(context.TODO())
	if err != nil {
		return err
	}

	// Get config and client for the k8s cluster
	err = e2e.buildConfigAndClientset()
	if err != nil {
		return err
	}

	// Wait for the cluster to have pods before we let the test suite run
	err = e2e.waitForHealthyCluster()

	return err
}

func (e2e *ZarfE2ETest) tearDownK3D() error {
	deleteClusterCommand := k3dcluster.NewCmdClusterDelete()
	err := deleteClusterCommand.ExecuteContext(context.TODO())
	os.Remove(e2e.kubeconfigPath)
	return err
}

func (e2e *ZarfE2ETest) setUpK3s() error {
	e2e.initWithK3s = true
	return nil
}

func (e2e *ZarfE2ETest) tearDownK3s() error {
	e2e.initWithK3s = false
	os.Remove(e2e.kubeconfigPath)
	return nil
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
	var err error

	if e2e.clientset == nil {
		err = e2e.buildConfigAndClientset()
		if err != nil {
			return "", "", err
		}
	}

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
	// Check if we need to deploy the k3s component
	if e2e.initWithK3s && commandString[0] == "init" {
		componentAdded := false
		for idx, str := range commandString {
			if strings.Contains(str, "components") {
				commandString[idx] = str + ",k3s"
				componentAdded = true
				break
			}
		}

		if !componentAdded {
			commandString = append(commandString, "--components=k3s")
		}
	}

	output, err := exec.Command(e2e.zarfBinPath, commandString...).CombinedOutput()
	return string(output), err
}

// Kill any background 'zarf connect ...' processes spawned during the tests
func (e2e *ZarfE2ETest) execZarfBackgroundCommand(commandString ...string) error {
	// Create a tunnel to the git resources
	tunnelCmd := exec.Command(e2e.zarfBinPath, commandString...)
	err := tunnelCmd.Start()
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	return err
}

// Check if any pods exist in the 'kube-system' namespace
func (e2e *ZarfE2ETest) checkIfClusterRunning() bool {
	err := e2e.buildConfigAndClientset()
	if err != nil {
		return false
	}

	pods, err := e2e.clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err == nil && len(pods.Items) > 0 {
		return true
	}

	return false
}

func (e2e *ZarfE2ETest) waitForHealthyCluster() error {
	attempt := 0
	for attempt < 10 {
		pods, err := e2e.clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
		if err == nil && len(pods.Items) >= 0 {
			allPodsHealthy := true

			// Make sure at the pods are in the 'succeeded' or 'running' state
			for _, pod := range pods.Items {
				if !(pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodRunning) {
					allPodsHealthy = false
					break
				}
			}

			if allPodsHealthy {
				fmt.Printf("💥 Cluster %s ready. You can access it by setting:\nexport KUBECONFIG='%s'\n", e2e.clusterName, e2e.kubeconfigPath)
				return nil
			}
		}

		time.Sleep(1 * time.Second)
		attempt++
	}

	return errors.New("unable to connect to cluster for e2e tests")
}
