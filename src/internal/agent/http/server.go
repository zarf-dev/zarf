package http

import (
	"fmt"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/agent/hooks"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// NewServer creates and return a http.Server
func NewServer(port string) *http.Server {
	message.Debugf("http.NewServer(%v)", port)

	// Instances hooks
	podsMutation := hooks.NewPodMutationHook()

	// Routers
	ah := newAdmissionHandler()
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/mutate/pods", ah.Serve(podsMutation))

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}
