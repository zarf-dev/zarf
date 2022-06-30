package test

import (
	"context"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
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

	t.Log("Running `zarf init`, limit to 10 minutes")
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Minute)
	defer cancel()

	// run `zarf init`
	_, _, err := utils.ExecCommandWithContext(ctx, true, e2e.zarfBinPath, "init", "--components="+initComponents, "--confirm")
	require.NoError(t, err)
}
