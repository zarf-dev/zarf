package api

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/cluster"
	"github.com/defenseunicorns/zarf/src/internal/api/packages"
	"github.com/defenseunicorns/zarf/src/internal/api/state"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// LaunchAPIServer launches UI API server
func LaunchAPIServer() {
	message.Debug("api.LaunchAPIServer()")

	// Token used to communicate with the API server
	token := utils.RandomString(96)

	// Init the Chi router
	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.NoCache)
	// @todo: bypass auth flow for now until we can make dev easier
	// router.Use(common.ValidateToken(token))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	router.Use(middleware.Timeout(60 * time.Second))

	router.Route("/api", func(r chi.Router) {

		r.Route("/cluster", func(r chi.Router) {
			r.Get("/", cluster.Summary)
			r.Get("/reachable", cluster.Reachable)
			r.Get("/has-zarf", cluster.HasZarf)
			r.Put("/initialize", cluster.InitializeCluster)
		})

		r.Route("/packages", func(r chi.Router) {
			r.Get("/find", packages.Find)
			r.Get("/find-in-home", packages.FindInHome)
			r.Get("/read/{path}", packages.Read)
			r.Get("/list", packages.ListDeployedPackages)
			r.Put("/deploy", packages.DeployPackage)
			r.Delete("/remove/{name}", packages.RemovePackage)
		})

		r.Route("/state", func(r chi.Router) {
			r.Get("/", state.Read)
			r.Put("/", state.Update)
		})
	})

	message.Infof("Zarf UI connection: http://127.0.0.1:3333/auth?token=%s", token)

	if sub, err := fs.Sub(config.UIAssets, "build/ui"); err != nil {
		message.Error(err, "Unable to load the embedded ui assets")
	} else {
		router.Handle("/*", http.FileServer(http.FS(sub)))
	}

	http.ListenAndServe(":3333", router)
}
