package packages

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
	"github.com/mholt/archiver/v3"
)

// Read reads a package from the local filesystem and writes the zarf.yaml json to the response.
func Read(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Read()")

	path := chi.URLParam(r, "path")

	if pkg, err := readPackage(w, path); err != nil {
		message.ErrorWebf(err, w, "Unable to read the package")
	} else {
		common.WriteJSONResponse(w, pkg, http.StatusOK)
	}
}

// ReadInit finds and reads a zarf init package from the local filesystem and writes the zarf.yaml json to the response.
func ReadInit(w http.ResponseWriter, r *http.Request) {
	// Assume the init package is in the current working directory
	initPackageName := fmt.Sprintf("zarf-init-%s.tar.zst", config.GetArch())

	// If the init package is not in the current working directory, check the directory of the executable
	if utils.InvalidPath(initPackageName) {
		executablePath, err := utils.GetFinalExecutablePath()
		if err != nil {
			message.ErrorWebf(err, w, "Unable to get the directory where the Zarf executable is located.")
		}

		executableDir := path.Dir(executablePath)
		config.DeployOptions.PackagePath = filepath.Join(executableDir, initPackageName)

		// If the init-package doesn't exist in the executable directory, suggest trying to download
		if utils.InvalidPath(config.DeployOptions.PackagePath) {
			// @todo - should we offer to download the init package here?
			message.ErrorWebf(err, w, "Unable to find the init package, ensure the init package %s is in the same directory as the Zarf executable.", initPackageName)
		}
	}

	// Read the init package
	if pkg, err := readPackage(w, initPackageName); err != nil {
		message.ErrorWebf(err, w, "Unable to read the package")
	} else {
		common.WriteJSONResponse(w, pkg, http.StatusOK)
	}
}

// internal function to read a package from the local filesystem
func readPackage(w http.ResponseWriter, path string) (pkg types.APIZarfPackage, err error) {
	pkg.Path = path

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return pkg, err
	}

	// Extract the archive
	err = archiver.Extract(path, config.ZarfYAML, tmpDir)
	if err != nil {
		return pkg, err
	}

	// Read the Zarf yaml
	configPath := filepath.Join(tmpDir, "zarf.yaml")
	err = utils.ReadYaml(configPath, &pkg.ZarfPackage)
	if err != nil {
		return pkg, err
	}

	return pkg, err
}
