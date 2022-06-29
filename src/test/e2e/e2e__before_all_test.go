package test

import (
	"os/exec"
	"testing"

	"github.com/defenseunicorns/zarf/src/test/e2e/clusters"
	"github.com/stretchr/testify/require"
)

func TestE2eInitCluster(t *testing.T) {
	initComponents := "k3s,logging,git-server"
	e2e.setup(t)
	defer e2e.teardown(t)

	// Final check to make sure we have a working k8s cluster, skipped if we are using K3s
	if e2e.distroToUse != clusters.DistroK3s {
		t.Log("Validating cluster connectivity")
		err := clusters.TryValidateClusterIsRunning()
		require.NoError(t, err, "unable to connect to a running k8s cluster")

		// Don't add k3s to the init arg
		initComponents = "logging,git-server"
	}

	// run `zarf init`
	t.Log("Running `zarf init`")
	output, err := exec.Command(e2e.zarfBinPath, "init", "--components="+initComponents, "--confirm", "--no-progress").CombinedOutput()
	require.NoError(t, err, string(output))
}
