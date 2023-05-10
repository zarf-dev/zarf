// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
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

// Explore returns all files in the given path if it exists and is inside the user's home directory.
// Optional query parameter: path, defaults to the user's home directory.
// Optional query parameter: init, if true, only return init packages.
func Explore(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Explore()")
	var pattern *regexp.Regexp
	encodedPath := r.URL.Query().Get("path")
	isInit := r.URL.Query().Get("init")
	path, err := url.PathUnescape(encodedPath)

	if isInit == "true" {
		pattern = initPackagesPattern
	} else {
		pattern = packagePattern
	}

	home, err := os.UserHomeDir()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get user home directory.")
		return
	}

	// If no path is provided, use the user's home directory
	if path == "" {
		path = home
	}

	// Ensure the path is inside users home directory and is valid
	if strings.Contains(path, home) == false || utils.InvalidPath(path) {
		message.ErrorCodeWebf(errors.New("Invalid path"), w, http.StatusNotFound, "Unable to explore %s", path)
		return
	}

	files, err := getExplorerFiles(path, pattern)

	if err != nil {
		message.ErrorCodeWebf(err, w, http.StatusNotFound, "Unable to retrieve files in %s", path)
	} else {
		explorer := types.APIExplorer{
			Dir:   path,
			Files: files,
		}
		common.WriteJSONResponse(w, explorer, http.StatusOK)
	}
}

func getExplorerFiles(dir string, pattern *regexp.Regexp) ([]types.APIExplorerFile, error) {
	matches := make([]types.APIExplorerFile, 0)

	files, err := os.ReadDir(dir)

	for _, file := range files {

		if !file.IsDir() {
			path := fmt.Sprintf("%s/%s", dir, file.Name())
			if pattern != nil {
				if len(pattern.FindStringIndex(path)) > 0 {

					matches = append(matches, types.APIExplorerFile{
						Path:  path,
						IsDir: false,
					})
				}
			}
		} else {
			matches = append(matches, types.APIExplorerFile{
				Path:  fmt.Sprintf("%s/%s", dir, file.Name()),
				IsDir: true,
			})
		}
	}
	return matches, err
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
