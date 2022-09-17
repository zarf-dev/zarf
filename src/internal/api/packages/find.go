package packages

import (
	"net/http"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

var pattern = regexp.MustCompile(`zarf-package-.*\.tar`)

func Find(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Find()")

	cwd, err := os.Getwd()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting current working directory")
	}

	files, _ := utils.RecursiveFileList(cwd, pattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}

func FindInHome(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInHome()")

	home, err := os.UserHomeDir()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting user home directory")
	}

	files, _ := utils.RecursiveFileList(home, pattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}
