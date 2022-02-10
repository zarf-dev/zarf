package agent

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/defenseunicorns/zarf/cli/internal/agent/http"
	"github.com/defenseunicorns/zarf/cli/internal/message"
)

// Heavinly influenced by https://github.com/douglasmakey/admissioncontroller and
// https://github.com/slackhq/simple-kubernetes-webhook

// We can hard-code these because we control the entire thing anyway
const (
	httpPort = "8443"
	tlscert  = "/etc/certs/tls.crt"
	tlskey   = "/etc/certs/tls.key"
)

func StartWebhook() {
	message.Debug("controller.StartWebhook()")

	server := http.NewServer(httpPort)
	go func() {
		if err := server.ListenAndServeTLS(tlscert, tlskey); err != nil {
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
