package test

import (
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestE2eExampleGame(t *testing.T) {

	//run `zarf init`
	output, err := exec.Command(e2e.zarfBinPath, "init", "--confirm").CombinedOutput()
	assert.NoError(t, err, string(output))

	// Deploy the game
	output, err = exec.Command(e2e.zarfBinPath, "package", "deploy", "../../build/zarf-package-appliance-demo-multi-games.tar.zst", "--confirm").CombinedOutput()
	assert.NoError(t, err, string(output))

	// Establish the port-forward into the game service
	cmd := exec.Command(e2e.zarfBinPath, "connect", "doom", "--local-port=22333")
	e2e.cmdsToKill = append(e2e.cmdsToKill, cmd)
	err = cmd.Start()
	assert.NoError(t, err, "unable to connect to the doom port-forward")

	// Give the port-forward a second to establish.
	// Since the `connect` command gets executed in the background, it can take a few milliseconds for the tunnel to be created
	time.Sleep(1 * time.Second)

	// Check that 'curl' returns something.
	// Right now we're just checking that `curl` returns 0. It can be enhanced by scraping the HTML that gets returned or something.
	resp, err := http.Get("http://127.0.0.1:22333?doom")
	assert.NoError(t, err, resp)
	assert.Equal(t, 200, resp.StatusCode)

	// Clean up after this test (incase other tests cases will be run afterwards)
	e2e.cleanupAfterTest(t)
}
