package packages

import (
	"net/http"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

var packagePattern = regexp.MustCompile(`zarf-package-.*\.tar`)
var initPattern = regexp.MustCompile(`zarf-init-.*\.tar\.zst`)

func Find(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Find()")

	cwd, err := os.Getwd()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting current working directory")
	}

	files, _ := utils.RecursiveFileList(cwd, packagePattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}

func FindInHome(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInHome()")

	home, err := os.UserHomeDir()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting user home directory")
	}

	files, _ := utils.RecursiveFileList(home, packagePattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}

// FindInitPackage returns all init packages anywhere down the directory tree of the working directory.
func FindInitPackage(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInitPackage()")
	cwd, err := os.Getwd()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting current working directory")
	}

	files, _ := utils.RecursiveFileList(cwd, initPattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}
