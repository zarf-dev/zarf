package test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {
	t.Log("E2E: Testing data injection")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-data-injection-demo-%s.tar", e2e.arch)

	// Limit this deploy to 5 minutes
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Minute)
	defer cancel()

	// Deploy the data injection example
	stdOut, stdErr, err := utils.ExecCommandWithContext(ctx, true, e2e.zarfBinPath, "package", "deploy", path, "--confirm", "-l=trace")
	require.NoError(t, err, stdOut, stdErr)

	// Get the data injection pod
	pods, err := k8s.GetPods("demo")
	require.NoError(t, err)
	require.Equal(t, len(pods.Items), 1)
	pod := pods.Items[0]

	kubectlOut, _ := exec.Command("kubectl", "-n", pod.Namespace, "exec", pod.Name, "--", "ls", "/test").Output()
	assert.Contains(t, string(kubectlOut), "this-is-an-example-file.txt")

	kubectlOut, _ = exec.Command("kubectl", "-n", pod.Namespace, "exec", pod.Name, "--", "ls", "/test/subdirectory-test").Output()
	assert.Contains(t, string(kubectlOut), "this-is-an-example-file.txt")
}
