package state

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// Read the zarf state secret from the cluster, if it exists.
func Read(w http.ResponseWriter, r *http.Request) {
	message.Debug("state.Read()")

	if data := k8s.LoadZarfState(); data.Distro == "" {
		common.WriteEmpty(w)
	} else {
		common.WriteJSONResponse(w, data)
	}
}

// Update the zarf state secret in the cluster.
func Update(w http.ResponseWriter, r *http.Request) {
	message.Debug("state.Update()")

	var data types.ZarfState
	// r.Body
	// common.ReadJSONRequest(r, &data)

	if err := k8s.SaveZarfState(data); err != nil {
		// common.WriteError(w, err)
	} else {
		common.WriteJSONResponse(w, data)
	}
}
