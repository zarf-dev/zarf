package test

import (
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {

	// run `zarf init`
	output, err := exec.Command(e2e.zarfBinPath, "init", "--components=gitops-service,logging", "--confirm").CombinedOutput()
	require.NoError(t, err, string(output))

	// Establish the port-forward into the gitea service; give the service a few seconds to come up since this is not a command we can retry
	time.Sleep(5 * time.Second)
	tunnelCmd := exec.Command(e2e.zarfBinPath, "connect", "git")
	err = tunnelCmd.Start()
	require.NoError(t, err, "unable to establish tunnel to git")
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	// Make sure Gitea comes up cleanly
	resp, err := http.Get("http://127.0.0.1:45003/explore/repos")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Establish the port-forward into the logging service
	tunnelCmd = exec.Command(e2e.zarfBinPath, "connect", "logging")
	err = tunnelCmd.Start()
	require.NoError(t, err, "unable to establish tunnel to logging")
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	// Make sure Grafana comes up cleanly
	resp, err = http.Get("http://127.0.0.1:45002/monitor/login")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	e2e.cleanupAfterTest(t)
}
