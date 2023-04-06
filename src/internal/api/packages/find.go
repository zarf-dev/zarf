// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

var packagePattern = regexp.MustCompile(`zarf-package.+\.tar\.zst$`)
var initPattern = regexp.MustCompile(`(?i).*init.*\.tar\.zst$`)

// Find returns all packages anywhere down the directory tree of the working directory.
func Find(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.Find()")
	findPackage(packagePattern, w, os.Getwd)
}

// FindInHome returns all packages in the user's home directory.
func FindInHome(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.FindInHome()")
	findPackage(packagePattern, w, os.UserHomeDir)
}

// FindInitPackage returns all init packages anywhere down the directory tree of the users home directory.
func FindInitPackage(w http.ResponseWriter, _ *http.Request) {
	message.Debug("packages.FindInitPackage()")
	findPackage(initPattern, w, os.UserHomeDir)
}

func findPackage(pattern *regexp.Regexp, w http.ResponseWriter, setDir func() (string, error)) {
	targetDir, err := setDir()
	if err != nil {
		message.ErrorWebf(err, w, "Error getting directory")
		return
	}

	files, err := recursiveFileListSkipPermissionErrors(targetDir, pattern)
	if err != nil || len(files) == 0 {
		pkgNotFoundMsg := fmt.Sprintf("Unable to locate the package: %s", pattern.String())
		message.ErrorWebf(err, w, pkgNotFoundMsg)
		return
	}
	common.WriteJSONResponse(w, files, http.StatusOK)
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths.
func recursiveFileListSkipPermissionErrors(dir string, pattern *regexp.Regexp) (files []string, err error) {
	const dotcharater = 46
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		// Return errors
		if err != nil {
			// skip files and dirs we don't have permission to view.
			if os.IsPermission(err) {
				return nil
			}
			return err
		}

		// Skip hidden directories
		if d.IsDir() && d.Name()[0] == dotcharater {
			return filepath.SkipDir
		}

		if !d.IsDir() {
			if pattern != nil {
				if len(pattern.FindStringIndex(path)) > 0 {
					files = append(files, path)
				}
			} else {
				files = append(files, path)
			}
		}

		return nil
	})
	return files, err
}
