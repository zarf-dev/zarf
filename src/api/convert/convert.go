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

// V1Alpha1PkgToV1Beta1 converts a v1alpha1 ZarfPackage to a v1beta1 ZarfPackage.
func V1Alpha1PkgToV1Beta1(pkg v1alpha1.ZarfPackage) v1beta1.ZarfPackage {
	generic := internalv1alpha1.ConvertToGeneric(pkg)
	return internalv1beta1.ConvertFromGeneric(generic)
}
