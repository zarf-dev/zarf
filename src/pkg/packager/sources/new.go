// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

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

func identifySourceType(pkgSrc string) string {
	if helpers.IsURL(pkgSrc) {
		parsed, _ := url.Parse(pkgSrc)
		return parsed.Scheme
	}

	if strings.Contains(pkgSrc, ".part000") {
		return "split"
	}

	if config.IsValidFileExtension(pkgSrc) {
		return "tarball"
	}

	return ""
}

// New returns a new PackageSource based on the provided package options.
func New(pkgOpts *types.ZarfPackageOptions, destination types.PackagePathsMap) (types.PackageSource, error) {
	var source types.PackageSource

	if !destination.KeyExists(types.BaseDir) {
		return nil, fmt.Errorf("destination base directory does not exist")
	}

	pkgSrc := pkgOpts.PackageSource

	switch identifySourceType(pkgSrc) {
	case "oci":
		remote, err := oci.NewOrasRemote(pkgSrc)
		if err != nil {
			return nil, err
		}
		source = &OCISource{destination, pkgOpts, remote}
	case "tarball":
		source = &TarballSource{destination, pkgOpts}
	case "http", "https":
		source = &URLSource{destination, pkgOpts}
	case "sget":
		message.Warn(lang.WarnSGetDeprecation)
		source = &URLSource{destination, pkgOpts}
	case "split":
		source = &SplitTarballSource{destination, pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", pkgSrc)
	}

	message.Debugf("Using %T for %q", source, pkgSrc)

	return source, nil
}
