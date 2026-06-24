// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package convert provides functions for converting between Zarf package API versions.
package convert

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
)

// PackageV1alpha1ToV1beta1 converts a v1alpha1 ZarfPackage to a v1beta1 Package.
func PackageV1alpha1ToV1beta1(pkg v1alpha1.ZarfPackage) v1beta1.Package {
	generic := internalv1alpha1.ConvertToGeneric(pkg)
	v1beta1Pkg := internalv1beta1.ConvertFromGeneric(generic)
	return v1beta1.SetDeprecatedFromGeneric(generic, v1beta1Pkg)
}

// PackageV1Beta1ToV1Alpha1 converts a v1beta1 Package to a v1alpha1 ZarfPackage.
func PackageV1Beta1ToV1Alpha1(pkg v1beta1.Package) v1alpha1.ZarfPackage {
	generic := internalv1beta1.ConvertToGeneric(pkg)
	return internalv1alpha1.ConvertFromGeneric(generic)
}
