package components

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
)

// ListDeployedPackages writes a list of packages that have been deployed to the connected cluster.
func ListDeployingComponents(w http.ResponseWriter, r *http.Request) {
	deployingPackages := config.GetDeployingComponents()
	common.WriteJSONResponse(w, deployingPackages, http.StatusOK)
}
