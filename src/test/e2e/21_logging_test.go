package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogginga(t *testing.T) {
	t.Log("E2E: Logging")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Establish the port-forward into the logging service
	err := e2e.execZarfBackgroundCommand("connect", "logging", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to logging")

	// Make sure Grafana comes up cleanly
	resp, err := http.Get("http://127.0.0.1:45002/monitor/login")
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-loki-stack",
	})	
}
