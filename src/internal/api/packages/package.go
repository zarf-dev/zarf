package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/cmd"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

func DeployPackage(w http.ResponseWriter, r *http.Request) {
	message.Debug("api.DeployPackage")

	args := []string{"--components=someComponent"}
	cmd.PackageDeployCmd.SetArgs(args)
	cmd.PackageDeployCmd.Execute()

	data := k8s.LoadZarfState()
	if data.Distro == "" {
		common.WriteEmpty(w)
	} else {
		common.WriteJSONResponse(w, data)
	}
}
