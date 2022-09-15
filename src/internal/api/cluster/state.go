package cluster

import (
	"net/http"

	"cuelang.org/go/cmd/cue/cmd/interfaces"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

func GetState(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.GetState()")

	data := k8s.LoadZarfState()

	if data.Distro == "" {
		common.WriteJSONResponse(w, common.EMPTY{})
	} else {
		common.WriteJSONResponse(w, data)
	}
}
