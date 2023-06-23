// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// FindInHomeStream returns all packages in the user's home directory.
// If the init query parameter is true, only init packages will be returned.
func FindInHomeStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	init := r.URL.Query().Get("init")
	regexp := packager.ZarfPackagePattern
	if init == "true" {
		regexp = packager.ZarfInitPattern
	}

	done := make(chan bool)
	go func() {
		// User home directory
		homePath, err := os.UserHomeDir()
		if err != nil {
			streamError(err, w)
		} else {
			// Recursively search for and stream packages in the home directory
			recursivePackageStream(homePath, regexp, w)
		}
		close(done)
	}()

	<-done
}

// FindInitStream finds and streams all init packages in the current working directory, the cache directory, and execution directory
func FindInitStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	done := make(chan bool)
	go func() {
		// stream init packages in the execution directory
		if execDir, err := utils.GetFinalExecutablePath(); err == nil {
			streamDirPackages(execDir, packager.ZarfInitPattern, w)
		} else {
			streamError(err, w)
		}

		// Cache directory
		cachePath := config.GetAbsCachePath()
		// Create the cache directory if it doesn't exist
		if utils.InvalidPath(cachePath) {
			if err := os.MkdirAll(cachePath, 0755); err != nil {
				streamError(err, w)
			}
		}
		streamDirPackages(cachePath, packager.ZarfInitPattern, w)

		// Find init packages in the current working directory
		if cwd, err := os.Getwd(); err == nil {
			streamDirPackages(cwd, packager.ZarfInitPattern, w)
		} else {
			streamError(err, w)
		}
		close(done)
	}()
	<-done
}

// FindPackageStream finds and streams all packages in the current working directory
func FindPackageStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	done := make(chan bool)

	go func() {
		if cwd, err := os.Getwd(); err == nil {
			streamDirPackages(cwd, packager.ZarfPackagePattern, w)
		} else {
			streamError(err, w)
		}
		close(done)
	}()

	<-done
	// Find init packages in the current working directory
}

// recursivePackageStream recursively searches for and streams packages in the given directory
func recursivePackageStream(dir string, pattern *regexp.Regexp, w http.ResponseWriter) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		// ignore files/dirs that it does not have permission to read
		if err != nil && os.IsPermission(err) {
			return nil
		}

		// Return error if the pattern is invalid
		if pattern == nil {
			return filepath.ErrBadPattern
		}

		// Return errors
		if err != nil {
			return err
		}

		if !d.IsDir() {
			if len(pattern.FindStringIndex(path)) > 0 {
				streamPackage(path, w)
			}
			// Skip the trash bin and hidden directories
		} else if utils.IsTrashBin(path) {
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		streamError(err, w)
	}
}

// streamDirPackages streams all packages in the given directory
func streamDirPackages(dir string, pattern *regexp.Regexp, w http.ResponseWriter) {
	files, err := os.ReadDir(dir)
	if err != nil {
		streamError(err, w)
	}
	for _, file := range files {
		if !file.IsDir() {
			path := fmt.Sprintf("%s/%s", dir, file.Name())
			if pattern != nil {
				if len(pattern.FindStringIndex(path)) > 0 {
					streamPackage(path, w)
				}
			}
		}
	}
}

// streamPackage streams the package at the given path
func streamPackage(path string, w http.ResponseWriter) {
	pkg, err := ReadPackage(path)
	if err != nil {
		streamError(err, w)
	} else {
		jsonData, err := json.Marshal(pkg)
		if err != nil {
			streamError(err, w)
		} else {
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			w.(http.Flusher).Flush()
		}
	}
}

// streamError streams the given error to the client
func streamError(err error, w http.ResponseWriter) {
	fmt.Fprintf(w, "data: %s\n\n", err.Error())
	w.(http.Flusher).Flush()
}
