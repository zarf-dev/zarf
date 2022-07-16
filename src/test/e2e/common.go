package test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath    string
	arch           string
	applianceMode  bool
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
}

// teardown actions for each test
func (e2e *ZarfE2ETest) teardown(t *testing.T) {
	t.Log("Test teardown")

	spinner := message.NewProgressSpinner("Remove test helm charts")
	for _, chart := range e2e.chartsToRemove {
		helm.RemoveChart(chart.namespace, chart.name, spinner)
	}
	spinner.Success()

	e2e.chartsToRemove = []ChartTarget{}
}

// execZarfCommand executes a Zarf command
func (e2e *ZarfE2ETest) execZarfCommand(commandString ...string) (string, string, error) {
	return utils.ExecCommandWithContext(context.TODO(), true, e2e.zarfBinPath, commandString...)
}

func (e2e *ZarfE2ETest) cleanFiles(files ...string) {
	for _, file := range files {
		_ = os.RemoveAll(file)
	}
}
