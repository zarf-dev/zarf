package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {
	t.Log("E2E: Data injection")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-data-injection-demo-%s.tar", e2e.arch)

	// Limit this deploy to 5 minutes
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Minute)
	defer cancel()

	// Deploy the data injection example
	stdOut, stdErr, err := utils.ExecCommandWithContext(ctx, true, e2e.zarfBinPath, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// verify the file and injection marker were created
	stdOut, stdErr, err = utils.ExecCommandWithContext(context.TODO(), true, "kubectl", "--namespace=demo", "logs", "--tail=5", "--selector=app=data-injection")
	require.NoError(t, err, stdOut, stdErr)
	assert.Contains(t, stdOut, "this-is-an-example-file.txt")
	assert.Contains(t, stdOut, ".zarf-injection-")

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "demo",
		name:      "zarf-raw-example-data-injection-pod",
	})
}
