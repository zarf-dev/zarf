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

	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect("doom", false)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HttpEndpoint())
	assert.NoError(t, err, resp)
	assert.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
