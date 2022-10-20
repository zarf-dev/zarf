package http

import (
	"fmt"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/agent/hooks"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// NewAdmissionServer creates and returns a http admission webhook server.
func NewAdmissionServer(port string) *http.Server {
	message.Debugf("http.NewAdmissionServer(%s)", port)

	// Instances hooks
	podsMutation := hooks.NewPodMutationHook()
	gitRepositoryMutation := hooks.NewGitRepositoryMutationHook()

	// Routers
	ah := newAdmissionHandler()
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/mutate/pod", ah.Serve(podsMutation))
	mux.Handle("/mutate/flux-gitrepository", ah.Serve(gitRepositoryMutation))

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}

// NewProxyServer creates and returns an http proxy server.
func NewProxyServer(port string) *http.Server {
	message.Debugf("http.NewHTTPProxy(%s)", port)

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/", ProxyHandler())

	return &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}
}

func healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
