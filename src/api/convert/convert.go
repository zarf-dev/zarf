// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package convert is the public conversion boundary between wire schemas and api.Package.
package convert

import (
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
)

// PackageFromV1alpha1 converts a v1alpha1 wire package to the normalized model.
func PackageFromV1alpha1(pkg v1alpha1.ZarfPackage) api.Package {
	return internalv1alpha1.PackageFromV1alpha1(pkg)
}

// PackageFromV1beta1 converts a v1beta1 wire package to the normalized model.
func PackageFromV1beta1(pkg v1beta1.Package) api.Package {
	return internalv1beta1.PackageFromV1beta1(pkg)
}

// PackageToV1alpha1 converts the normalized model to the v1alpha1 wire package.
func PackageToV1alpha1(pkg api.Package) v1alpha1.ZarfPackage {
	return internalv1alpha1.PackageToV1alpha1(pkg)
}

// PackageToV1beta1 converts the normalized model to the v1beta1 wire package.
func PackageToV1beta1(pkg api.Package) v1beta1.Package {
	return internalv1beta1.PackageToV1beta1(pkg)
}

// PackageV1alpha1ToV1beta1 converts a v1alpha1 wire package to v1beta1.
func PackageV1alpha1ToV1beta1(pkg v1alpha1.ZarfPackage) v1beta1.Package {
	return PackageToV1beta1(PackageFromV1alpha1(pkg))
}

// PackageV1beta1ToV1alpha1 converts a v1beta1 wire package to v1alpha1.
func PackageV1beta1ToV1alpha1(pkg v1beta1.Package) v1alpha1.ZarfPackage {
	return PackageToV1alpha1(PackageFromV1beta1(pkg))
}
