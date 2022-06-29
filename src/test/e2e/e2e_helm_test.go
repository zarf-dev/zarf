package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2eExampleHelm(t *testing.T) {
	path := fmt.Sprintf("../../../build/zarf-package-test-helm-releasename-%s.tar.zst", e2e.arch)

	// Deploy the charts
	output, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)

	// Verify multiple helm installs of different release names were deployed
	kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-A", "--selector=app.kubernetes.io/name=nginx", "--no-headers").Output()
	assert.Contains(t, string(kubectlOut), "zarf-nginx-2")
	assert.Contains(t, string(kubectlOut), "zarf-nginx-3")
	assert.Contains(t, string(kubectlOut), "zarf-nginx-4")
}
