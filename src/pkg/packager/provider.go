// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"net/url"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

func identifySourceType(source string) string {
	if helpers.IsURL(source) {
		if helpers.IsOCIURL(source) {
			return "oci"
		}
		parsed, _ := url.Parse(source)
		if !isValidFileExtension(source) {
			return ""
		}
		switch parsed.Scheme {
		case "https":
			return "https"
		case "http":
			return "http"
		case "file":
			return "file"
		default:
			return ""
		}
	}

	if utils.InvalidPath(source) {
		return ""
	}

	if isValidFileExtension(source) {
		return "tarball"
	}

	// TODO: handle partial packages

	return ""
}

func ProviderFromSource(pkgOpts *types.ZarfPackageOptions, destination string) (types.PackageProvider, error) {
	var provider types.PackageProvider

	source := pkgOpts.PackagePath

	switch identifySourceType(source) {
	case "oci":
		message.Debug("Identified source as OCI")
		provider = &OCIProvider{source: source, destinationDir: destination, opts: pkgOpts}
		remote, err := oci.NewOrasRemote(source)
		if err != nil {
			return nil, err
		}
		remote.WithInsecureConnection(config.CommonOptions.Insecure)
		provider.(*OCIProvider).OrasRemote = remote
	case "tarball":
		message.Debug("Identified source as tarball")
		provider = &TarballProvider{source: source, destinationDir: destination, opts: pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", source)
	}

	return provider, nil
}
