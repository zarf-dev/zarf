package api

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/api/cluster"
	"github.com/defenseunicorns/zarf/src/internal/message"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// LaunchAPIServer launches UI API server
func LaunchAPIServer() {
	message.Debug("api.LaunchAPIServer()")

	rotuer := chi.NewRouter()

	rotuer.Use(middleware.Logger)
	rotuer.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	rotuer.Use(middleware.Timeout(60 * time.Second))

	rotuer.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	rotuer.Route("/api/cluster", func(r chi.Router) {
		r.Get("/state", cluster.GetState)
	})

	http.ListenAndServe(":3333", rotuer)
}
