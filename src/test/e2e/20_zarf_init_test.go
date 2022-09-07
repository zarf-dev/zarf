package test

import (
	"context"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestZarfInit(t *testing.T) {
	t.Log("E2E: Zarf init (limit to 10 minutes)")
	e2e.setup(t)
	defer e2e.teardown(t)

	initComponents := "logging,git-server"
	// Add k3s compoenent in appliance mode
	if e2e.applianceMode {
		initComponents = "k3s,logging,git-server"
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Minute)
	defer cancel()

	// run `zarf init`
	_, _, err := utils.ExecCommandWithContext(ctx, true, e2e.zarfBinPath, "init", "--components="+initComponents, "--confirm")
	require.NoError(t, err)

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = utils.ExecCommandWithContext(ctx, true, "kubectl", "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
	_, _, _ = utils.ExecCommandWithContext(ctx, true, "kubectl", "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
	_, _, _ = utils.ExecCommandWithContext(ctx, true, "kubectl", "set", "resources", "deploy", "-n", "zarf", "zarf-docker-registry", "--limits=cpu=200m")
}
