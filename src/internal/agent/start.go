package agent

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	agentHttp "github.com/defenseunicorns/zarf/src/internal/agent/http"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// Heavily influenced by https://github.com/douglasmakey/admissioncontroller and
// https://github.com/slackhq/simple-kubernetes-webhook

// We can hard-code these because we control the entire thing anyway
const (
	httpPort = "8443"
	tlscert  = "/etc/certs/tls.crt"
	tlskey   = "/etc/certs/tls.key"
)

// StartWebhook launches the zarf agent mutating webhook in the cluster
func StartWebhook() {
	message.Debug("agent.StartWebhook()")

	server := agentHttp.NewServer(httpPort)
	go func() {
		if err := server.ListenAndServeTLS(tlscert, tlskey); err != nil && err != http.ErrServerClosed {
			message.Fatal(err, "Failed to start the web server")
		}
	}()

	message.Infof("Server running in port: %s", httpPort)

	// listen shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	message.Infof("Shutdown gracefully...")
	if err := server.Shutdown(context.Background()); err != nil {
		message.Fatal(err, "unable to properly shutdown the web server")
	}
}
