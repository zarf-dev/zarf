package packages

import (
	"net/http"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi"
	"github.com/mholt/archiver/v3"
)

func Read(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Read()")

	path := chi.URLParam(r, "path")
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.ErrorWebf(err, w, "Error creating temp directory")
	}

	// Extract the archive
	err = archiver.Extract(path, config.ZarfYAML, tmpDir)
	if err != nil {
		message.ErrorWebf(err, w, "Error extracting the zarf package")
	}

	configPath := filepath.Join(tmpDir, "zarf.yaml")
	var pkg types.ZarfPackage
	err = utils.ReadYaml(configPath, &pkg)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to read the config file in the package")
	}

	common.WriteJSONResponse(w, pkg, http.StatusOK)
}
