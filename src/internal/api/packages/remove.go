package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/go-chi/chi/v5"
)

// RemovePackage removes a package that has been deployed to the cluster.
func RemovePackage(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := packager.Remove(name)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to remove the zarf package from the cluster")
		return
	}

	common.WriteJSONResponse(w, nil, http.StatusOK)
}
