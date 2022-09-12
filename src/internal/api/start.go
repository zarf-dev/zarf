package api

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/cluster"
	"github.com/defenseunicorns/zarf/src/internal/message"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// LaunchAPIServer launches UI API server
func LaunchAPIServer() {
	message.Debug("api.LaunchAPIServer()")

	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	router.Use(middleware.Timeout(60 * time.Second))

	router.Route("/api/cluster", func(r chi.Router) {
		r.Get("/state", cluster.GetState)
	})

	if sub, err := fs.Sub(config.UIAssets, "build/ui"); err != nil {
		message.Error(err, "Unable to load the embedded ui assets")
	} else {
		router.Handle("/*", http.FileServer(http.FS(sub)))
	}

	http.ListenAndServe(":3333", router)
}
