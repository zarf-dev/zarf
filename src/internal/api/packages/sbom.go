// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
	"github.com/mholt/archiver/v3"
)

var signalChan = make(chan os.Signal, 1)
var filePaths = make(map[string]string)

// ExtractSBOM Extracts the SBOM from the package and returns the path to the SBOM
func ExtractSBOM(w http.ResponseWriter, r *http.Request) {
	path := chi.URLParam(r, "path")

	sbom, err := extractSBOM(path)

	if err != nil {
		message.ErrorWebf(err, w, err.Error())
	} else {
		common.WriteJSONResponse(w, sbom, http.StatusOK)
	}

}

func DeleteSBOM(w http.ResponseWriter, r *http.Request) {
	err := cleanupSBOM()
	if err != nil {
		message.ErrorWebf(err, w, err.Error())
		return
	}
	common.WriteJSONResponse(w, nil, http.StatusOK)
}

// cleanupSBOM removes the SBOM directory
func cleanupSBOM() error {
	err := os.RemoveAll(config.ZarfSBOMDir)
	if err != nil {
		return err
	}
	filePaths = make(map[string]string)
	return nil
}

// Extracts the SBOM from the package and returns the path to the SBOM
func extractSBOM(escapedPath string) (sbom types.APIPackageSBOM, err error) {
	const sbomDir = "zarf-sbom"
	const SBOM = "sboms.tar"

	path, err := url.QueryUnescape(escapedPath)
	if err != nil {
		return sbom, err
	}

	// Check if the SBOM has already been extracted
	if filePaths[path] != "" {
		sbom, err = getSbomViewFiles(filePaths[path])
	} else {
		if err != nil {
			return sbom, err
		}
		// ensure the package exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return sbom, err
		}

		// Create the Zarf SBOM directory
		tmpDir, err := utils.MakeTempDir(config.ZarfSBOMDir)
		if err != nil {
			return sbom, err
		}

		// Extract the SBOM tar.gz from the package
		err = archiver.Extract(path, SBOM, tmpDir)
		if err != nil {
			cleanupSBOM()
			return sbom, err
		}

		// Unarchive the SBOM tar.gz
		err = archiver.Unarchive(filepath.Join(tmpDir, SBOM), tmpDir)
		if err != nil {
			cleanupSBOM()
			return sbom, err
		}

		// Get the SBOM viewer files
		sbom, err = getSbomViewFiles(tmpDir)
		if err != nil {
			cleanupSBOM()
			return sbom, err
		}

		// Cleanup the temp directory on exit
		go func() {
			signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
			// Wait for a signal to be received
			<-signalChan

			cleanupSBOM()

			// Exit the program
			os.Exit(0)
		}()

		filePaths[path] = tmpDir
	}
	return sbom, err
}

func getSbomViewFiles(sbomPath string) (sbom types.APIPackageSBOM, err error) {
	cwd, _ := os.Getwd()
	sbomViewFiles, err := filepath.Glob(filepath.Join(sbomPath, "sbom-viewer-*"))
	if len(sbomViewFiles) > 0 {
		sbom.Path = filepath.Join(cwd, sbomViewFiles[0])
		sbom.SBOMS = sbomViewFiles
	}
	return sbom, err
}
