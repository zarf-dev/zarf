package test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath string

	arch          string
	clusterName   string
	filesToRemove []string
	cmdsToKill    []*exec.Cmd
	restConfig    *restclient.Config
	clientset     *kubernetes.Clientset
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
	e2e.restConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}
	e2e.clientset, err = kubernetes.NewForConfig(e2e.restConfig)

	return err
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
	if shouldCreateK3sCluster() && commandString[0] == "init" {
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

// Return true if the user wants to use the built-in K3s, false otherwise
func shouldCreateK3sCluster() bool {
	return os.Getenv("TESTDISTRO") == "k3s"
}
