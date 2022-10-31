package packages

import (
	"encoding/json"
	"net/http"
	"path"
	"path/filepath"

	globalConfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/packager"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// DeployPackage deploys a package to the Zarf cluster.
func DeployPackage(w http.ResponseWriter, r *http.Request) {
	isInitPkg := r.URL.Query().Get("isInitPkg") == "true"

	config := types.PackagerConfig{}

	if isInitPkg {
		var body = types.ZarfInitOptions{}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
			return
		}
		config.IsInitConfig = true
		config.InitOpts = body
		initPackageName := packager.GetInitPackageName("")
		config.DeployOpts.PackagePath = initPackageName

		// Try to use an init-package in the executable directory if none exist in current working directory
		if utils.InvalidPath(config.DeployOpts.PackagePath) {
			// Get the path to the executable
			if executablePath, err := utils.GetFinalExecutablePath(); err != nil {
				message.Errorf(err, "Unable to get the path to the executable")
			} else {
				executableDir := path.Dir(executablePath)
				config.DeployOpts.PackagePath = filepath.Join(executableDir, initPackageName)
			}

			// If the init-package doesn't exist in the executable directory, try the cache directory
			if err != nil || utils.InvalidPath(config.DeployOpts.PackagePath) {
				config.DeployOpts.PackagePath = filepath.Join(globalConfig.GetAbsCachePath(), initPackageName)

				// If the init-package doesn't exist in the cache directory, return an error
				if utils.InvalidPath(config.DeployOpts.PackagePath) {
					common.WriteJSONResponse(w, false, http.StatusBadRequest)
					return
				}
			}
		}
	} else {
		var body = types.ZarfDeployOptions{}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			message.ErrorWebf(err, w, "Unable to decode the request to deploy the cluster")
			return
		}
		config.DeployOpts = body
	}

	globalConfig.CommonOptions.Confirm = true

	pkg, err := packager.NewPackager(&config)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to deploy the zarf package to the cluster")
	}

	if err := pkg.Deploy(); err != nil {
		message.ErrorWebf(err, w, "Unable to deploy the zarf package to the cluster")
		return
	}

	common.WriteJSONResponse(w, true, http.StatusCreated)
}
