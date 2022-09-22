package api

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/cluster"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/api/packages"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// LaunchAPIServer launches UI API server
func LaunchAPIServer() {
	message.Debug("api.LaunchAPIServer()")

	// Track the developer port if it's set
	devPort := os.Getenv("API_DEV_PORT")

	// If the env variable API_PORT is set, use that for the listening port
	port := os.Getenv("API_PORT")
	// Otherwise, use a random available port
	if port == "" {
		// If we can't find an available port, just use the default
		if portRaw, err := k8s.GetAvailablePort(); err != nil {
			port = "8080"
		} else {
			port = fmt.Sprintf("%d", portRaw)
		}
	}

	// If the env variable API_TOKEN is set, use that for the API secret
	token := os.Getenv("API_TOKEN")
	// Otherwise, generate a random secret
	if token == "" {
		token = utils.RandomString(96)
	}

	// Init the Chi router
	router := chi.NewRouter()

	// Push logs into the message buffer for log persistence
	genericMsg := message.Generic{}
	logFormatter := middleware.DefaultLogFormatter{
		Logger: log.New(&genericMsg, "API CALL | ", log.LstdFlags),
	}

	router.Use(middleware.RequestLogger(&logFormatter))
	router.Use(middleware.Recoverer)
	router.Use(middleware.NoCache)

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	router.Use(middleware.Timeout(60 * time.Second))

	router.Route("/api", func(r chi.Router) {
		// Require a valid token for API calls
		r.Use(common.RequireAuthSecret(token))

		r.Route("/cluster", func(r chi.Router) {
			r.Get("/", cluster.Summary)

			r.Route("/state", func(r chi.Router) {
				r.Get("/", cluster.ReadState)
				r.Put("/", cluster.UpdateState)
			})
		})

		r.Route("/packages", func(r chi.Router) {
			r.Get("/find", packages.Find)
			r.Get("/find-in-home", packages.FindInHome)
			r.Get("/find-init", packages.FindInitPackage)
			r.Get("/read/{path}", packages.Read)
			r.Get("/list", packages.ListDeployedPackages)
			r.Put("/deploy", packages.DeployPackage)
			r.Delete("/remove/{name}", packages.RemovePackage)
		})

	})

	// If no dev port specified, use the server port for the URL and try to open it
	if devPort == "" {
		url := fmt.Sprintf("http://127.0.0.1:%s/auth?token=%s", port, token)
		message.Infof("Zarf UI connection: %s", url)
		message.Debug(utils.ExecLaunchURL(url))
	} else {
		// Otherwise, use the dev port for the URL and don't try to open
		message.Infof("Zarf UI connection: http://127.0.0.1:%s/auth?token=%s", devPort, token)
	}

	// Load the static UI files
	if sub, err := fs.Sub(config.UIAssets, "build/ui"); err != nil {
		message.Error(err, "Unable to load the embedded ui assets")
	} else {
		// Setup a file server for the static UI files
		fs := http.FileServer(http.FS(sub))

		// Catch all routes
		router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// If the request is not a real file, serve the index.html instead
			if test, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/")); err != nil {
				r.URL.Path = "/"
			} else {
				test.Close()
			}
			fs.ServeHTTP(w, r)
		})
	}

	http.ListenAndServe(":"+port, router)
}
