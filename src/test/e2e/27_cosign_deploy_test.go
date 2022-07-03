package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: Cosign deploy")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Test with command from https://zarf.dev/install/
	command := fmt.Sprintf("%s package deploy sget://defenseunicorns/zarf-hello-world:$(uname -m) --confirm", e2e.zarfBinPath)

	stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), true, "sh", "-c", command)
	require.NoError(t, err, stdOut, stdErr)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-raw-multi-games",
	})
}
