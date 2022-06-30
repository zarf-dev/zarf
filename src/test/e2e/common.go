package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/test/e2e/clusters"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath string
	arch        string
	distroToUse clusters.DistroToUse
	cmdsToKill  []*exec.Cmd
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

// setup actions for each test
func (e2e *ZarfE2ETest) setup(t *testing.T) {
	t.Log("Test setup")
}

// teardown actions for each test
func (e2e *ZarfE2ETest) teardown(t *testing.T) {
	t.Log("Test teardown")
	// Kill background processes spawned during the test
	for _, cmd := range e2e.cmdsToKill {
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("unable to kill process: %v", err)
			}
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

// execZarfCommand executes a Zarf command
func (e2e *ZarfE2ETest) execZarfCommand(commandString ...string) (string, string, error) {
	return utils.ExecCommandWithContext(context.TODO(), true, e2e.zarfBinPath, commandString...)
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

// Get the pods from the provided namespace
func (e2e *ZarfE2ETest) getPodsFromNamespace(namespace string) (*v1.PodList, error) {

	metaOptions := metav1.ListOptions{}
	clientset, _ := clusters.GetClientSet()
	tries := 0
	for tries < 10 {
		pods, _ := clientset.CoreV1().Pods(namespace).List(context.TODO(), metaOptions)
		if pods.Items[0].Status.Phase == v1.PodRunning {
			return pods, nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil, errors.New("unable to get a healthy pod from the namespace")
}

func (e2e *ZarfE2ETest) cleanFiles(files ...string) {
	for _, file := range files {
		_ = os.Remove(file)
	}
}
