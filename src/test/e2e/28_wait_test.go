package test

import (
	// "context"
	"fmt"
	"time"

	// "os/exec"
	"testing"

	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type zarfCommandResult struct {
	stdOut string
	stdErr string
	err    error
}

func zarfCommandWStruct(e2e ZarfE2ETest, path string) (result zarfCommandResult) {
	result.stdOut, result.stdErr, result.err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	return result
}

func TestWait(t *testing.T) {
	t.Log("E2E: Helm Wait")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-test-helm-wait-%s.tar.zst", e2e.arch)

	zarfChannel := make(chan zarfCommandResult, 1)
	go func() {
		zarfChannel <- zarfCommandWStruct(e2e, path)
	}()

	var stdOut string
	var stdErr string
	var err error

	select{
	case res := <-zarfChannel:
		stdOut = res.stdOut
		stdErr = res.stdErr
		err    = res.err
	case <-time.After(10 * time.Second):
		t.Error("Timeout waiting for zarf deploy (it tried to wait)")
	}

	// Deploy the charts
	// stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// // Verify multiple helm installs of different release names were deployed
	// kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-n=helm-releasename", "--no-headers").Output()
	// assert.Contains(t, string(kubectlOut), "zarf-cool-name-podinfo")

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "test-helm-wait", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}