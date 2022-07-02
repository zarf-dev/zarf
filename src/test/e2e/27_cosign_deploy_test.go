package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: Cosign deploy")
	e2e.setup(t)
	defer e2e.teardown(t)

	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", "sget://defenseunicorns/zarf-hello-world:$(uname -m)", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-raw-multi-games",
	})
}
