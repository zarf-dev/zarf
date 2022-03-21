package test

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2eExampleComposability(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	//run `zarf init`
	output, err := e2e.execZarfCommand("init", "--confirm")
	require.NoError(t, err, output)

	path := fmt.Sprintf("../../build/zarf-package-compose-example-%s.tar.zst", e2e.arch)

	// Deploy the composable game package
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)

	// Establish the port-forward into the game service
	err = e2e.execZarfBackgroundCommand("connect", "doom", "--local-port=22333", "--cli-only")
	require.NoError(t, err, "unable to connect to the doom port-forward")

	// Right now we're just checking that `curl` returns 0. It can be enhanced by scraping the HTML that gets returned or something.
	resp, err := http.Get("http://127.0.0.1:22333?doom")
	require.NoError(t, err, resp)

	// Read the body into string
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, body)

	// Validate the doom title in body.
	assert.Contains(t, string(body), "Zarf needs games too")
	assert.Equal(t, 200, resp.StatusCode)
}
