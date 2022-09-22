package packages

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

var packagePattern = regexp.MustCompile(`zarf-package-.*\.tar`)
var initPattern = regexp.MustCompile(fmt.Sprintf("zarf-init-%s.tar.zst", config.GetArch()))

// Find returns all packages anywhere down the directory tree of the working directory.
func Find(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Find()")
	findPackage(packagePattern, w, os.Getwd)
}

// FindInHome returns all packages in the user's home directory.
func FindInHome(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInHome()")
	findPackage(packagePattern, w, os.UserHomeDir)
}

// FindInitPackage returns all init packages anywhere down the directory tree of the working directory.
func FindInitPackage(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.FindInitPackage()")
	findPackage(initPattern, w, os.Getwd)
}

func findPackage(pattern *regexp.Regexp, w http.ResponseWriter, setDir func() (string, error)) {
	targetDir, err := setDir()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting directory")
	}

	// Intentionally ignore errors
	files, _ := utils.RecursiveFileList(targetDir, pattern)
	common.WriteJSONResponse(w, files, http.StatusOK)
}
