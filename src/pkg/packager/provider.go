// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
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
		case utils.SGETURLScheme:
			return "sget"
		default:
			return ""
		}
	}

	if utils.InvalidPath(source) {
		return ""
	}

	if strings.Contains(source, ".part000") {
		return "partial"
	}

	if isValidFileExtension(source) {
		return "tarball"
	}

	return ""
}

func ProviderFromSource(pkgOpts *types.ZarfPackageOptions, destination string) (types.PackageProvider, error) {
	var provider types.PackageProvider

	source := pkgOpts.PackagePath

	switch identifySourceType(source) {
	case "oci":
		message.Debug("Identified source", source, "as OCI package")
		provider = &OCIProvider{source: source, destinationDir: destination, opts: pkgOpts}
		remote, err := oci.NewOrasRemote(source)
		if err != nil {
			return nil, err
		}
		remote.WithInsecureConnection(config.CommonOptions.Insecure)
		provider.(*OCIProvider).OrasRemote = remote
	case "tarball":
		message.Debug("Identified source", source, "as tarball package")
		provider = &TarballProvider{source: source, destinationDir: destination, opts: pkgOpts}
	case "http", "https":
		message.Debug("Identified source", source, "as HTTP(S) package")
		provider = &URLProvider{source: source, destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "sget":
		message.Debug("Identified source", source, "as SGET package")
		message.Warn(lang.WarnSGetDeprecation)
		provider = &URLProvider{source: source, destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "partial":
		message.Debug("Identified source", source, "as partial package")
		provider = &PartialTarballProvider{source: source, destinationDir: destination, opts: pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", source)
	}

	return provider, nil
}
