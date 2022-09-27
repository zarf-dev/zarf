package test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/utils"
)

// ZarfE2ETest Struct holding common fields most of the tests will utilize
type ZarfE2ETest struct {
	zarfBinPath   string
	arch          string
	applianceMode bool
}

// getCLIName looks at the OS and CPU architecture to determine which Zarf binary needs to be run
func GetCLIName() string {
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
	utils.ExecCommandWithContext(context.TODO(), "", true, "sh", "-c", "kubectl describe nodes |grep -A 99 Non\\-terminated")
}

// teardown actions for each test
func (e2e *ZarfE2ETest) teardown(t *testing.T) {
	t.Log("Test teardown")
}

// execZarfCommand executes a Zarf command
func (e2e *ZarfE2ETest) execZarfCommand(commandString ...string) (string, string, error) {
	return utils.ExecCommandWithContext(context.TODO(), "", true, e2e.zarfBinPath, commandString...)
}

func (e2e *ZarfE2ETest) cleanFiles(files ...string) {
	for _, file := range files {
		_ = os.RemoveAll(file)
	}
}
