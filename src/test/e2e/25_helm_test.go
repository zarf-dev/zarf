package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelm(t *testing.T) {
	t.Log("E2E: Helm chart")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-test-helm-releasename-%s.tar.zst", e2e.arch)

	// Deploy the charts
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify multiple helm installs of different release names were deployed
	kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-n=helm-releasename", "--no-headers").Output()
	assert.Contains(t, string(kubectlOut), "zarf-cool-name-podinfo")

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "demo",
		name:      "zarf-raw-example-data-injection-pod",
	})
}
