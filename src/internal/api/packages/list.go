package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// ListDeployedPackages writes a list of packages that have been deployed to the connected cluster.
func ListDeployedPackages(w http.ResponseWriter, r *http.Request) {
	deployedPackages, err := k8s.GetDeployedZarfPackages()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get a list of the deployed Zarf packages")

		return
	}

	common.WriteJSONResponse(w, deployedPackages, http.StatusOK)
}
