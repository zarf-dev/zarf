// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/zoci"
	"github.com/defenseunicorns/zarf/src/types"
)

// PackageSource is an interface for package sources.
//
// While this interface defines three functions, LoadPackage, LoadPackageMetadata, and Collect; only one of them should be used within a packager function.
//
// These functions currently do not promise repeatability due to the side effect nature of loading a package.
//
// Signature and integrity validation is up to the implementation of the package source.
//
//	`sources.ValidatePackageSignature` and `sources.ValidatePackageIntegrity` can be leveraged for this purpose.
type PackageSource interface {
	// LoadPackage loads a package from a source.
	LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg types.ZarfPackage, warnings []string, err error)

	// LoadPackageMetadata loads a package's metadata from a source.
	LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg types.ZarfPackage, warnings []string, err error)

	// Collect relocates a package from its source to a tarball in a given destination directory.
	Collect(ctx context.Context, destinationDirectory string) (tarball string, err error)
}

// Identify returns the type of package source based on the provided package source string.
func Identify(src string) string {
	if helpers.IsURL(src) {
		parsed, _ := url.Parse(src)
		return parsed.Scheme
	}

	if strings.Contains(src, ".part000") {
		return "split"
	}

	if IsValidFileExtension(src) {
		return "tarball"
	}

	return ""
}

// New returns a new PackageSource based on the provided package options.
func New(src, shasum, publicKeyPath, sGetKeyPath string) (PackageSource, error) {
	var source PackageSource
	switch Identify(src) {
	case "oci":
		ociSrc := src
		if shasum != "" {
			ociSrc = fmt.Sprintf("%s@sha256:%s", src, shasum)
		}
		arch := config.GetArch()
		remote, err := zoci.NewRemote(ociSrc, oci.PlatformForArch(arch))
		if err != nil {
			return nil, err
		}
		source = &OCISource{
			Remote:        remote,
			Src:           ociSrc,
			PublicKeyPath: publicKeyPath,
		}
	case "tarball":
		source = &TarballSource{
			Src:           src,
			Shasum:        shasum,
			PublicKeyPath: publicKeyPath,
		}
	case "http", "https", "sget":
		source = &URLSource{
			Src:           src,
			Shasum:        shasum,
			PublicKeyPath: publicKeyPath,
			SGetKeyPath:   sGetKeyPath,
		}
	case "split":
		source = &SplitTarballSource{
			Src:           src,
			Shasum:        shasum,
			PublicKeyPath: publicKeyPath,
		}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", src)
	}
	message.Debugf("Using %T for %q", source, src)
	return source, nil
}
