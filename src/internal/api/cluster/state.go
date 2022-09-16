package cluster

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

// GetState loads the zarf-state secret from the k8s cluster
func GetState(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.GetState()")

	data := k8s.LoadZarfState()

	if data.Distro == "" {
		common.WriteEmpty(w)
	} else {
		common.WriteJSONResponse(w, 200, data)
	}
}

// RunZarfConnect creates a portforward connection tunnel to the designated resource
func RunZarfConnect(w http.ResponseWriter, r *http.Request) {
	r.URL.Query()
	common.WriteJSONResponse(w, 501, "Not Implemented")
}

// // KillZarfConnect
// func KillZarfConnect(w http.ResponseWriter, r *http.Request) {
// 	common.WriteJSONResponse(w, 501, "Not Implemented")
// }

func GetVersion(w http.ResponseWriter, r *http.Request) {
	common.WriteJSONResponse(w, 200, config.CLIVersion)
}

func GetGitPassword(w http.ResponseWriter, r *http.Request) {
	state := k8s.LoadZarfState()
	config.InitState(state)
	gitPassword := config.GetSecret(config.StateGitPush)

	common.WriteJSONResponse(w, 501, gitPassword)
}

// GetConnectionOptions outputs all potential `zarf connect` command strings based on what has been deployed to the cluster
func GetConnectionOptions(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.GetConnectionOptions()")

	connectStrings, err := k8s.GetConnectStrings()
	if err != nil {
		common.WriteJSONResponse(w, 500, err)
	}

	common.WriteJSONResponse(w, 200, connectStrings)
}
