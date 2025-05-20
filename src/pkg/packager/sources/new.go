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
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"
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
	LoadPackage(ctx context.Context, dst *layout.PackagePaths, filter filters.ComponentFilterStrategy, unarchiveAll bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error)

	// LoadPackageMetadata loads a package's metadata from a source.
	LoadPackageMetadata(ctx context.Context, dst *layout.PackagePaths, wantSBOM bool, skipValidation bool) (pkg v1alpha1.ZarfPackage, warnings []string, err error)

	// Collect relocates a package from its source to a tarball in a given destination directory.
	Collect(ctx context.Context, destinationDirectory string) (tarball string, err error)
}

// Identify returns the type of package source based on the provided package source string.
func Identify(pkgSrc string) string {
	if helpers.IsURL(pkgSrc) {
		parsed, _ := url.Parse(pkgSrc)
		return parsed.Scheme
	}

	if strings.Contains(pkgSrc, ".part000") {
		return "split"
	}

	if IsValidFileExtension(pkgSrc) {
		return "tarball"
	}

	return ""
}

// New returns a new PackageSource based on the provided package options.
func New(ctx context.Context, pkgOpts *types.ZarfPackageOptions) (PackageSource, error) {
	var source PackageSource

	pkgSrc := pkgOpts.PackageSource

	switch Identify(pkgSrc) {
	case "oci":
		if pkgOpts.Shasum != "" {
			pkgSrc = fmt.Sprintf("%s@sha256:%s", pkgSrc, pkgOpts.Shasum)
		}
		arch := config.GetArch()
		remote, err := zoci.NewRemote(ctx, pkgSrc, oci.PlatformForArch(arch))
		if err != nil {
			return nil, err
		}
		source = &OCISource{ZarfPackageOptions: pkgOpts, Remote: remote}
	case "tarball":
		source = &TarballSource{pkgOpts}
	case "http", "https", "sget":
		source = &URLSource{pkgOpts}
	case "split":
		source = &SplitTarballSource{pkgOpts}
	default:
		return nil, fmt.Errorf("could not identify source type for %q", pkgSrc)
	}

	return source, nil
}
