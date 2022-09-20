package cluster

import (
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
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
