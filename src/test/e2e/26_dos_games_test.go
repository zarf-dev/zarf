package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDosGames(t *testing.T) {
	t.Log("E2E: Dos games")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-dos-games-%s.tar.zst", e2e.arch)

	// Deploy the game
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Get a random local port for this instance
	localPort, _ := k8s.GetAvailablePort()

	// Establish the port-forward into the game service
	err = e2e.execZarfBackgroundCommand("connect", "doom", fmt.Sprintf("--local-port=%d", localPort), "--cli-only")
	require.NoError(t, err, "unable to connect to the doom port-forward")

	// Check that 'curl' returns something.
	// Right now we're just checking that `curl` returns 0. It can be enhanced by scraping the HTML that gets returned or something.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d?doom", localPort))
	assert.NoError(t, err, resp)
	assert.Equal(t, 200, resp.StatusCode)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-raw-multi-games",
	})
}
