// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
	goyaml "github.com/goccy/go-yaml"
	"github.com/mholt/archiver/v3"
)

// Read reads a package from the local filesystem and writes the Zarf.yaml json to the response.
func Read(w http.ResponseWriter, r *http.Request) {
	message.Debug("packages.Read()")

	path := chi.URLParam(r, "path")

	if pkg, err := readPackage(path); err != nil {
		message.ErrorWebf(err, w, "Unable to read the package at: `%s`", path)
	} else {
		common.WriteJSONResponse(w, pkg, http.StatusOK)
	}
}

// internal function to read a package from the local filesystem.
func readPackage(path string) (pkg types.APIZarfPackage, err error) {
	var file []byte

	pkg.Path, err = url.QueryUnescape(path)
	if err != nil {
		return pkg, err
	}

	// Check for zarf.yaml in the package and read into file
	err = archiver.Walk(pkg.Path, func(f archiver.File) error {
		if f.Name() == config.ZarfYAML {
			file, err = ioutil.ReadAll(f)
			if err != nil {
				return err
			} else {
				return archiver.ErrStopWalk
			}
		}

		return nil
	})
	if err != nil {
		return pkg, err
	}

	err = goyaml.Unmarshal(file, &pkg.ZarfPackage)
	return pkg, err
}
