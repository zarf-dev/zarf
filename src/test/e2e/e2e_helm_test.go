package test

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2eExampleHelm(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	//run `zarf init`
	output, err := e2e.execZarfCommand("init", "--confirm")
	require.NoError(t, err, output)

	path := fmt.Sprintf("../../../build/zarf-package-test-helm-releasename-%s.tar.zst", e2e.arch)

	// Deploy the charts
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "-l=trace")
	require.NoError(t, err, output)

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "get", "pods", "-A", "--selector=app.kubernetes.io/name=etcd", "--no-headers").Output()
	assert.Contains(t, string(kubectlOut), "zarf-etcd-0")
	assert.Contains(t, string(kubectlOut), "zarf-etcd-2-0")
	assert.Contains(t, string(kubectlOut), "zarf-etcd-3-0")
	assert.Contains(t, string(kubectlOut), "zarf-etcd-4-0")
}
