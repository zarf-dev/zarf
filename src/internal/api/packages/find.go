// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Find zarf-packages on the local system (https://regex101.com/r/TUUftK/1)
var packagePattern = regexp.MustCompile(`zarf-package[^\s\\\/]*\.tar(\.zst)?$`)
// Find zarf-init packages on the local system (https://regex101.com/r/6aTl3O/2)
var initPattern = regexp.MustCompile(`zarf-init[^\s\\\/]*\.tar(\.zst)?$`)

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

	// Skip permission errors, search dot-prefixed directories.
	files, err := utils.RecursiveFileList(targetDir, pattern, true, false)
	if err != nil || len(files) == 0 {
		pkgNotFoundMsg := fmt.Sprintf("Unable to locate the package: %s", pattern.String())
		message.ErrorWebf(err, w, pkgNotFoundMsg)
		return
	}
	common.WriteJSONResponse(w, files, http.StatusOK)
}
