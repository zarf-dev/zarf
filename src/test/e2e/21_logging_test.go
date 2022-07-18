package test

import (
	"net/http"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func TestLogging(t *testing.T) {
	t.Log("E2E: Logging")
	e2e.setup(t)
	defer e2e.teardown(t)

	tunnel := k8s.NewZarfTunnel()
	tunnel.Connect(k8s.ZarfLogging, false)
	defer tunnel.Close()

	// Make sure Grafana comes up cleanly
	resp, err := http.Get(tunnel.HttpEndpoint())
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	e2e.chartsToRemove = append(e2e.chartsToRemove, ChartTarget{
		namespace: "zarf",
		name:      "zarf-loki-stack",
	})
}
