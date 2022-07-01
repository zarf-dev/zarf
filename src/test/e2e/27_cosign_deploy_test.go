package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: Cosign deploy")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("sget://defenseunicorns/zarf-hello-world:%s", e2e.arch)

	// Deploy the game
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-raw-multi-games",
	})	
}
