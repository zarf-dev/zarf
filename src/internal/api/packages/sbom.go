// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
)

var signalChan = make(chan os.Signal, 1)
var filePaths = make(map[string]string)

// ExtractSBOM Extracts the SBOM from the package and returns the path to the SBOM
func ExtractSBOM(w http.ResponseWriter, r *http.Request) {
	var body types.APIZarfPackage
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		message.ErrorWebf(err, w, "Unable to decode the requested package")
		return
	}
	sbom, err := extractSBOM(&body)

	if err != nil {
		message.ErrorWebf(err, w, err.Error())
	} else {
		common.WriteJSONResponse(w, sbom, http.StatusOK)
	}

}

// Extracts the SBOM from the package and returns the path to the SBOM
func extractSBOM(pkg *types.APIZarfPackage) (sbom types.APIPackageSBOM, err error) {
	const sbomDir = "zarf-sbom"
	const SBOM = "sboms.tar"

	path := pkg.Path
	name := pkg.ZarfPackage.Metadata.Name

	// Check if the SBOM has already been extracted
	if filePaths[name] != "" {
		sbom, err = getSbomViewFiles(filePaths[name])
	} else {
		// Get the current working directory
		cwd, err := os.UserHomeDir()
		if err != nil {
			return sbom, err
		}
		// ensure the package exists
		if _, err := os.Stat(pkg.Path); os.IsNotExist(err) {
			return sbom, err
		}
		tmpPath := filepath.Join(cwd, sbomDir, name)

		// tmpSBOMs := filepath.Join(config.CommonOptions.TempDirectory, "sboms")
		tmpDir, err := utils.MakeTempDir(tmpPath)
		if err != nil {
			return sbom, err
		}
		cleanup := func() {
			os.RemoveAll(tmpDir)
		}

		// Extract the SBOM tar.gz from the package
		err = archiver.Extract(path, SBOM, tmpDir)
		if err != nil {
			cleanup()
			return sbom, err
		}

		// Unarchive the SBOM tar.gz
		err = archiver.Unarchive(filepath.Join(tmpDir, SBOM), tmpDir)
		if err != nil {
			cleanup()
			return sbom, err
		}

		// Get the SBOM viewer files
		sbom, err = getSbomViewFiles(tmpDir)
		if err != nil {
			cleanup()
			return sbom, err
		}

		// Cleanup the temp directory on exit
		go func() {
			signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
			// Wait for a signal to be received
			<-signalChan

			// Call the cleanup function
			os.RemoveAll(filepath.Join(cwd, sbomDir))

			// Exit the program
			os.Exit(0)
		}()

		filePaths[name] = tmpDir
	}
	return sbom, err
}

func getSbomViewFiles(sbomPath string) (sbom types.APIPackageSBOM, err error) {
	sbomViewFiles, err := filepath.Glob(filepath.Join(sbomPath, "sbom-viewer-*"))
	if len(sbomViewFiles) > 0 {
		sbom.Path = sbomViewFiles[0]
		sbom.SBOMS = sbomViewFiles
	}
	return sbom, err
}
