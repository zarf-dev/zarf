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

	var state types.ZarfState
	var reachable bool
	var distro string
	var hasZarf bool

	if err := k8s.WaitForHealthyCluster(5 * time.Second); err == nil {
		reachable = true
	}

	if reachable {
		distro, _ = k8s.DetectDistro()
		state, _ = k8s.LoadZarfState()
		hasZarf = state.Distro != ""
	}

	data := types.ClusterSummary{
		Reachable: reachable,
		HasZarf:   hasZarf,
		Distro:    distro,
		ZarfState: state,
	}

	common.WriteJSONResponse(w, data, http.StatusOK)
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
