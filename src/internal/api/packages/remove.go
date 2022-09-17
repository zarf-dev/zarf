package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/go-chi/chi/v5"
)

// RemovePackage removes a package that has been deployed to the cluster.
func RemovePackage(w http.ResponseWriter, r *http.Request) {
	// Get the components to remove from the (optional) query params
	components := r.URL.Query().Get("components")
	config.DeployOptions.Components = components

	// Get the name of the package we're removing from the URL params
	name := chi.URLParam(r, "name")

	// Remove the package
	err := packager.Remove(name)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to remove the zarf package from the cluster")
		return
	}

	common.WriteJSONResponse(w, nil, http.StatusOK)
}
