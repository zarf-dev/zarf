package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func TestLogginga(t *testing.T) {
	t.Log("E2E: Logging")
	e2e.setup(t)
	defer e2e.teardown(t)

	// Get a random local port for this instance
	localPort, _ := k8s.GetAvailablePort()

	// Establish the port-forward into the logging service
	err := e2e.execZarfBackgroundCommand("connect", "logging", fmt.Sprintf("--local-port=%d", localPort), "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to logging")

	// Make sure Grafana comes up cleanly
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/monitor/login", localPort))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-loki-stack",
	})
}
