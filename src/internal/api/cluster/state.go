package cluster

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

func GetState(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.GetState()")

	data := k8s.LoadZarfState()

	if data.Distro == "" {
		common.WriteEmpty(w)
	} else {
		common.WriteJSONResponse(w, data)
	}
}
