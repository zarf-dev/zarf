// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package providers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

func identifySourceType(source string) string {
	if helpers.IsURL(source) {
		parsed, _ := url.Parse(source)
		return parsed.Scheme
	}

	if strings.Contains(source, ".part000") {
		return "partial"
	}

	if config.IsValidFileExtension(source) {
		return "tarball"
	}

	return ""
}

func NewFromSource(pkgOpts *types.ZarfPackageOptions, destination string) (types.PackageProvider, error) {
	var provider types.PackageProvider

	source := pkgOpts.PackagePath

	switch identifySourceType(source) {
	case "oci":
		provider = &OCIProvider{source: source, destinationDir: destination, opts: pkgOpts}
		remote, err := oci.NewOrasRemote(source)
		if err != nil {
			return nil, err
		}
		remote.WithInsecureConnection(config.CommonOptions.Insecure)
		provider.(*OCIProvider).OrasRemote = remote
	case "tarball":
		provider = &TarballProvider{source: source, destinationDir: destination, opts: pkgOpts}
	case "http", "https":
		provider = &URLProvider{source: source, destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "sget":
		message.Warn(lang.WarnSGetDeprecation)
		provider = &URLProvider{source: source, destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "partial":
		provider = &PartialTarballProvider{source: source, destinationDir: destination, opts: pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", source)
	}

	message.Debugf("Using %T for %q", provider, source)

	return provider, nil
}
