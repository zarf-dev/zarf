package packages

import (
	"encoding/json"
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/defenseunicorns/zarf/src/types"
)

// DeployPackage deploys a package to the Zarf cluster.
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	// Decode the body of the request
	var deployClusterRequest = types.ZarfDeployOptions{}
	err := json.NewDecoder(r.Body).Decode(&deployClusterRequest)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")

		return
	}

	// Set the deploy options and deploy!
	config.DeployOptions = deployClusterRequest
	config.CommonOptions.Confirm = true
	packager.Deploy()

	common.WriteJSONResponse(w, true, http.StatusCreated)
}
