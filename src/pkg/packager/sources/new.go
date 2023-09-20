// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// PackageSource is an interface for package sources.
//
// While this interface defines two functions, LoadPackage and LoadPackageMetadata, only one of them should be used within a packager function.
//
// These functions currently do not promise repeatability due to the side effect nature of loading a package.
type PackageSource interface {
	// LoadPackage loads a package from a source.
	//
	// For the default sources included in Zarf, package integrity (checksums, signatures, etc.) is validated during this function
	// and expects the package structure to follow the default Zarf package structure.
	//
	// If your package does not follow the default Zarf package structure, you will need to implement your own source.
	LoadPackage(*layout.PackagePaths) error
	// LoadPackageMetadata loads a package's metadata from a source.
	//
	// This function follows the same principles as LoadPackage, with a few exceptions:
	//
	// - Package integrity validation will display a warning instead of returning an error if
	//   the package is signed but no public key is provided. This is to allow for the inspection and removal of packages
	//   that are signed but the user does not have the public key for.
	LoadPackageMetadata(dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) error

	// Collect relocates a package from its source to a destination tarball.
	Collect(string) error
}

func Identify(pkgSrc string) string {
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
func New(pkgOpts *types.ZarfPackageOptions) (PackageSource, error) {
	var source PackageSource

	pkgSrc := pkgOpts.PackageSource

	switch Identify(pkgSrc) {
	case "oci":
		remote, err := oci.NewOrasRemote(pkgSrc)
		if err != nil {
			return nil, err
		}
		source = &OCISource{pkgOpts, remote}
	case "tarball":
		source = &TarballSource{pkgOpts}
	case "http", "https", "sget":
		source = &URLSource{pkgOpts}
	case "split":
		source = &SplitTarballSource{pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", pkgSrc)
	}

	message.Debugf("Using %T for %q", source, pkgSrc)

	return source, nil
}
