package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiteaAndGrafana(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	// run `zarf init`
	output, err := e2e.execZarfCommand("init", "--components=gitops-service,logging", "--confirm")
	require.NoError(t, err, output)

	// Establish the port-forward into the gitea service; give the service a few seconds to come up since this is not a command we can retry
	err = e2e.execZarfBackgroundCommand("connect", "git", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to git")

	// Make sure Gitea comes up cleanly
	resp, err := http.Get("http://127.0.0.1:45003/explore/repos")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Establish the port-forward into the logging service
	err = e2e.execZarfBackgroundCommand("connect", "logging", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to logging")

	// Make sure Grafana comes up cleanly
	resp, err = http.Get("http://127.0.0.1:45002/monitor/login")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
