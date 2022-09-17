package cluster

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/defenseunicorns/zarf/src/types"
)

// Summary returns a summary of cluster status.
func Summary(w http.ResponseWriter, r *http.Request) {
	message.Debug("cluster.Summary()")

	data := types.ClusterSummary{
		Reachable: reachable(),
		HasZarf:   hasZarf(),
		Distro:    distro(),
	}

	common.WriteJSONResponse(w, data, http.StatusOK)
}

// Reachable checks if we can connect to the cluster.
func Reachable(w http.ResponseWriter, r *http.Request) {
	message.Debug("cluster.Reachable()")
	common.WriteJSONResponse(w, reachable(), http.StatusOK)
}

// HasZarf checks if the cluster has been initialized by Zarf.
func HasZarf(w http.ResponseWriter, r *http.Request) {
	message.Debug("cluster.HasZarf()")
	common.WriteJSONResponse(w, hasZarf(), http.StatusOK)
}

func reachable() bool {
	// Test if we can connect to the cluster.
	err := k8s.WaitForHealthyCluster(5 * time.Second)
	return err == nil
}

func hasZarf() bool {
	data, err := k8s.LoadZarfState()
	if err != nil {
		return false
	}

	// If this is an empty zarf state, then the cluster hasn't been initialized yet
	return data.Distro != ""
}

func distro() string {
	if distro, err := k8s.DetectDistro(); err != nil {
		return ""
	} else {
		return distro
	}
}

// InitializeCluster initializes the connected k8s cluster with Zarf
// This is equivalent to running the `zarf init` command.
func InitializeCluster(w http.ResponseWriter, r *http.Request) {
	var initializeClusterRequest = types.ZarfDeployOptions{}
	err := json.NewDecoder(r.Body).Decode(&initializeClusterRequest)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the request to initialize the cluster")

		return
	}

	config.DeployOptions = initializeClusterRequest
	config.CommonOptions.Confirm = true
	packager.Deploy()

	common.WriteJSONResponse(w, true, http.StatusCreated)
}
