// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

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
		return "partial"
	}

	if config.IsValidFileExtension(pkgSrc) {
		return "tarball"
	}

	return ""
}

func New(pkgOpts *types.ZarfPackageOptions, destination string) (types.PackageSource, error) {
	var source types.PackageSource

	pkgSrc := pkgOpts.PackageSource

	switch identifySourceType(pkgSrc) {
	case "oci":
		source = &OCISource{destinationDir: destination, opts: pkgOpts}
		remote, err := oci.NewOrasRemote(pkgSrc)
		if err != nil {
			return nil, err
		}
		source.(*OCISource).OrasRemote = remote
	case "tarball":
		source = &TarballSource{destinationDir: destination, opts: pkgOpts}
	case "http", "https":
		source = &URLSource{destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "sget":
		message.Warn(lang.WarnSGetDeprecation)
		source = &URLSource{destinationDir: destination, opts: pkgOpts, insecure: config.CommonOptions.Insecure}
	case "partial":
		source = &PartialTarballSource{destinationDir: destination, opts: pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", pkgSrc)
	}

	message.Debugf("Using %T for %q", source, pkgSrc)

	return source, nil
}
