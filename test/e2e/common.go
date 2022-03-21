package test

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/test/e2e/clusters"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath   string
	arch          string
	distroToUse   clusters.DistroToUse
	filesToRemove []string
	cmdsToKill    []*exec.Cmd
}

// getCLIName looks at the OS and CPU architecture to determine which Zarf binary needs to be run
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

// cleanupAfterTest cleans up after a test run so that the next test can run in a clean environment. It needs to be
// deferred at the beginning of each test. Example:
//
// func TestE2eFooBarBaz(t *testing.T) {
//     defer e2e.cleanupAfterTest(t)
//     doAllTheOtherStuff...
// }
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

// execCommandInPod does the equivalent of `kubectl exec` to run one or more shell commands inside a pod.
// It returns the stdout and stderr, and an error if anything went wrong
func (e2e *ZarfE2ETest) execCommandInPod(podname string, namespace string, cmd []string) (string, string, error) {
	stdoutBuffer := &strings.Builder{}
	stderrBuffer := &strings.Builder{}
	var err error

	clientSet, err := clusters.GetClientSet()
	if err != nil {
		return "", "", fmt.Errorf("unable to connect to cluster: %w", err)
	}
	req := clientSet.CoreV1().RESTClient().Post().Resource("pods").Name(podname).Namespace(namespace).SubResource("exec")
	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	req.VersionedParams(option, scheme.ParameterCodec)

	config, err := clusters.GetConfig()
	if err != nil {
		return "", "", err
	}
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: stdoutBuffer,
		Stderr: stderrBuffer,
	})

	return stdoutBuffer.String(), stderrBuffer.String(), err
}

// execZarfCommand executes a Zarf command. It automatically knows which Zarf binary to use, and it has special logic
// That adds the "k3s" component if the user wants to use the build-in K3s and `zarf init` is the command being run.
// It requires
func (e2e *ZarfE2ETest) execZarfCommand(commandString ...string) (string, error) {
	// TODO: It might be a nice feature to read some flag/env and change the stdout and stderr to pipe to the terminal running the test

	// Check if we need to deploy the k3s component
	if e2e.distroToUse == clusters.DistroK3s && commandString[0] == "init" {
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

// execZarfBackgroundCommand kills any background 'zarf connect ...' processes spawned during the tests
func (e2e *ZarfE2ETest) execZarfBackgroundCommand(commandString ...string) error {
	// Create a tunnel to the git resources
	tunnelCmd := exec.Command(e2e.zarfBinPath, commandString...)
	err := tunnelCmd.Start()
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	return err
}
