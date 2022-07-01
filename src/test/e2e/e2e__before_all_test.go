package test

import (
	"context"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestE2eInitCluster(t *testing.T) {
	e2e.setup(t)
	defer e2e.teardown(t)
	
	initComponents := "logging,git-server"
	// Add k3s compoenent in appliance mode
	if e2e.applianceMode {
		initComponents = "k3s,logging,git-server"
	}

	t.Log("Running `zarf init`, limit to 10 minutes")
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Minute)
	defer cancel()

	// run `zarf init`
	_, _, err := utils.ExecCommandWithContext(ctx, true, e2e.zarfBinPath, "init", "--components="+initComponents, "--confirm")
	require.NoError(t, err)
}
