// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Find zarf-packages on the local system (https://regex101.com/r/TUUftK/1)
var packagePattern = regexp.MustCompile(`zarf-package[^\s\\\/]*\.tar(\.zst)?$`)

// Find zarf-init packages on the local system
var currentInitPattern = regexp.MustCompile(packager.GetInitPackageName(""))

// Find any zarf-init package on the local system (https://regex101.com/r/6aTl3O/2)
var initPackagesPattern = regexp.MustCompile(`zarf-init[^\s\\\/]*\.tar(\.zst)?$`)

// Find return all package paths in the current working directory.
func Find(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.Find()")
	path, err := os.Getwd()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get current working directory")
		return
	}
	files, err := findFilePaths(packagePattern, path)
	if err != nil {
		message.ErrorCodeWebf(err, w, http.StatusNotFound, "Unable to find ZarfPackages in current working directory.")
	} else {
		common.WriteJSONResponse(w, files, http.StatusOK)
	}
}

// FindInHome returns all package paths in the user's home directory.
func FindInHome(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.FindInHome()")
	path, err := os.UserHomeDir()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get user home directory.")
	}
	message.Debug("packages.FindInHome()")

	files, err := findFilePaths(packagePattern, path)
	if err != nil {
		message.ErrorCodeWebf(err, w, http.StatusNotFound, "Unable to find ZarfPackages in user home directory.")
		return
	}
	common.WriteJSONResponse(w, files, http.StatusOK)
}

// FindInitPackage returns all init package paths in the current working directory, the cache directory, and the user's, and the execution directory
func FindInitPackage(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.FindInitPackage()")
	var errs error
	files := make([]string, 0)

	// Find init packages in the execution directory
	if execDir, err := os.Getwd(); err == nil {
		filesExecDir, err := findFilePaths(currentInitPattern, execDir)
		if err != nil {
			errs = fmt.Errorf("%s", err)
		} else {
			files = append(files, filesExecDir...)
		}
	}

	// Cache directory
	cachePath := config.GetAbsCachePath()
	// Create the cache directory if it doesn't exist
	if utils.InvalidPath(cachePath) {
		if err := os.MkdirAll(cachePath, 0755); err != nil {
			message.Fatalf(err, lang.CmdInitErrUnableCreateCache, cachePath)
		}
	}
	// Look for init packages in the cache directory
	cacheFiles, err := findFilePaths(currentInitPattern, cachePath)
	if err != nil {
		errs = fmt.Errorf("%s\n%s", errs, err)
	} else {
		files = append(files, cacheFiles...)
	}

	// Find init packages in the current working directory
	if cwd, err := os.Getwd(); err == nil {
		cwdPackages, err := findFilePaths(currentInitPattern, cwd)
		if err != nil {
			errs = fmt.Errorf("%s\n%s", errs, err)
		} else {
			files = append(files, cwdPackages...)
		}
	}

	// If any files exist return them, otherwise return an error
	if len(files) > 0 {
		common.WriteJSONResponse(w, files, http.StatusFound)
	} else {
		message.ErrorCodeWebf(errs, w, http.StatusNotFound, "Unable to find ZarfInitPackages.")
	}
}

// findFilePaths returns all files matching the pattern in the given path.
func findFilePaths(pattern *regexp.Regexp, path string) ([]string, error) {
	// Find all files matching the pattern
	files, err := utils.FileList(path, pattern)

	if err != nil || len(files) == 0 {
		pkgNotFoundMsg := fmt.Errorf("Unable to locate packages at %s matching %s.", path, pattern.String())

		return nil, pkgNotFoundMsg
	}
	return files, nil
}
