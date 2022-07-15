package test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath    string
	arch           string
	applianceMode  bool
	cmdsToKill     []*exec.Cmd
	chartsToRemove []ChartTarget
}

type ChartTarget struct {
	name      string
	namespace string
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
	// Output list of allocated cluster resources
	utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", "kubectl describe nodes |grep -A 99 Non\\-terminated")
	// List currently listening ports on the host
	utils.ExecCommandWithContext(context.TODO(), true, "lsof", "-iTCP", "-sTCP:LISTEN", "-n")
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

	spinner := message.NewProgressSpinner("Remove test helm charts")
	for _, chart := range e2e.chartsToRemove {
		helm.RemoveChart(chart.namespace, chart.name, spinner)
	}
	spinner.Success()

	e2e.cmdsToKill = []*exec.Cmd{}
	e2e.chartsToRemove = []ChartTarget{}
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

	timeout := time.After(5 * time.Second)
	for {
		// Delay the first check by 1 second
		time.Sleep(1 * time.Second)
		select {

		// on timeout abort
		case <-timeout:
			return errors.New("Timeout reached while waiting for background command to start")

		default:
			// Check if the command has started yet
			// NOTE: ExitCode() returns -1 if the process hasn't started yet!
			if tunnelCmd.Process != nil && tunnelCmd.ProcessState != nil && tunnelCmd.ProcessState.ExitCode() != -1 {
				// The background process seems to be running..
				return err
			}
		}
	}
}

func (e2e *ZarfE2ETest) cleanFiles(files ...string) {
	for _, file := range files {
		_ = os.RemoveAll(file)
	}
}
