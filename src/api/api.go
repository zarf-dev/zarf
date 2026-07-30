// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package api supplies helpers for working generically with the different API versions
package api

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalTypes "github.com/zarf-dev/zarf/src/internal/api/types"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
)

// PackageAccessor is the read contract for a package source, exposing a per-version definition.
type PackageAccessor interface {
	AsV1alpha1() v1alpha1.ZarfPackage
	AsV1beta1() v1beta1.Package
}

// PackageDefinition is a concrete package source backed by the internal generic package representation.
type PackageDefinition struct {
	pkg internalTypes.Package
}

var _ PackageAccessor = PackageDefinition{}

// NewPackageDefinitionFromV1alpha1 creates a PackageDefinition from a v1alpha1 package definition.
func NewPackageDefinitionFromV1alpha1(pkg v1alpha1.ZarfPackage) PackageDefinition {
	return PackageDefinition{pkg: internalv1alpha1.ConvertToGeneric(pkg)}
}

// NewPackageDefinitionFromV1beta1 creates a PackageDefinition from a v1beta1 package definition.
func NewPackageDefinitionFromV1beta1(pkg v1beta1.Package) PackageDefinition {
	return PackageDefinition{pkg: internalv1beta1.ConvertToGeneric(pkg)}
}

// AsV1alpha1 returns the package definition as a v1alpha1 ZarfPackage.
func (p PackageDefinition) AsV1alpha1() v1alpha1.ZarfPackage {
	return internalv1alpha1.ConvertFromGeneric(p.pkg)
}

// AsV1beta1 returns the package definition as a v1beta1 Package.
func (p PackageDefinition) AsV1beta1() v1beta1.Package {
	return internalv1beta1.ConvertFromGeneric(p.pkg)
}

// SetName sets the package metadata name.
func (p *PackageDefinition) SetName(name string) {
	p.pkg.Metadata.Name = name
}

// SetAnnotations sets the package metadata annotations.
func (p *PackageDefinition) SetAnnotations(annotations map[string]string) {
	p.pkg.Metadata.Annotations = annotations
}
