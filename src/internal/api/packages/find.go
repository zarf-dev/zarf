package packages

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
)

func Find(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Find()")
	
	files, _ := filepath.Glob(config.PackagePrefix + "-*.tar*")
	common.WriteJSONResponse(w, files, http.StatusOK)
}

func FindInHome(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInHome()")

	homePath, _ := os.UserHomeDir()
	path := filepath.Join(homePath, config.PackagePrefix)
	files, _ := filepath.Glob(path + "-*.tar*")
	common.WriteJSONResponse(w, files, http.StatusOK)
}
