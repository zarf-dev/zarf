package cluster

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// GetDeployedPackages looks for secrets with the prefix 'zarf-package-' within the zarf namespace and returns their data
func GetDeployedPackages(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.GetDeployedPackages()")
	common.WriteJSONResponse(w, 501, "Not Implemented")
}

// InitializeCluster initializes the
func InitializeCluster(w http.ResponseWriter, r *http.Request) {
	common.WriteJSONResponse(w, 501, "Not Implemented")

}

// DeployPackage deploys a package...
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	common.WriteJSONResponse(w, 501, "Not Implemented")

}

// RemovePackage removes an already deployed package
func RemovePackage(w http.ResponseWriter, r *http.Request) {
	common.WriteJSONResponse(w, 501, "Not Implemented")

}
