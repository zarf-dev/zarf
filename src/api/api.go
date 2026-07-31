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

// OriginalAPIVersion returns the apiVersion the package was authored in before any conversion.
func (p PackageDefinition) OriginalAPIVersion() string {
	return p.pkg.Build.OriginalAPIVersion
}

// SetName sets the package metadata name.
func (p *PackageDefinition) SetName(name string) {
	p.pkg.Metadata.Name = name
}

// SetAnnotations sets the package metadata annotations.
func (p *PackageDefinition) SetAnnotations(annotations map[string]string) {
	p.pkg.Metadata.Annotations = annotations
}

// SelectComponents keeps only components with the given names.
func (p *PackageDefinition) SelectComponents(names []string) {
	byName := make(map[string][]internalTypes.Component, len(p.pkg.Components))
	for _, component := range p.pkg.Components {
		byName[component.Name] = append(byName[component.Name], component)
	}
	components := p.pkg.Components[:0]
	for _, name := range names {
		matches := byName[name]
		if len(matches) == 0 {
			continue
		}
		components = append(components, matches[0])
		byName[name] = matches[1:]
	}
	p.pkg.Components = components
}

// SetComponentImages updates the image list for the named component.
func (p *PackageDefinition) SetComponentImages(componentName string, images []string) {
	for i := range p.pkg.Components {
		if p.pkg.Components[i].Name != componentName {
			continue
		}
		p.pkg.Components[i].Images = mergeImageSources(p.pkg.Components[i].Images, images)
		return
	}
}

// SetComponentRepositories updates the git repository list for the named component.
func (p *PackageDefinition) SetComponentRepositories(componentName string, repos []string) {
	for i := range p.pkg.Components {
		if p.pkg.Components[i].Name != componentName {
			continue
		}
		p.pkg.Components[i].Repositories = repositoriesFromV1alpha1(repos)
		return
	}
}

func mergeImageSources(existing []internalTypes.Image, names []string) []internalTypes.Image {
	sourceByName := make(map[string]string, len(existing))
	for _, image := range existing {
		sourceByName[image.Name] = image.Source
	}
	images := make([]internalTypes.Image, 0, len(names))
	for _, name := range names {
		images = append(images, internalTypes.Image{Name: name, Source: sourceByName[name]})
	}
	return images
}

func repositoriesFromV1alpha1(repos []string) []internalTypes.Repository {
	out := make([]internalTypes.Repository, 0, len(repos))
	for _, repo := range repos {
		out = append(out, internalTypes.Repository{URL: repo})
	}
	return out
}

// SetAggregateChecksum records the generated checksums.txt aggregate checksum in every API location.
func (p *PackageDefinition) SetAggregateChecksum(checksum string) {
	p.pkg.Metadata.AggregateChecksum = checksum
	p.pkg.Build.AggregateChecksum = checksum
}

// SetBuildMetadataFromV1alpha1 records generated build fields from the canonical v1alpha1 view.
func (p *PackageDefinition) SetBuildMetadataFromV1alpha1(pkg v1alpha1.ZarfPackage) {
	p.pkg.Metadata.Version = pkg.Metadata.Version
	p.pkg.Build.Hostname = pkg.Build.Terminal
	p.pkg.Build.User = pkg.Build.User
	p.pkg.Build.Architecture = pkg.Build.Architecture
	p.pkg.Build.Timestamp = pkg.Build.Timestamp
	p.pkg.Build.Version = pkg.Build.Version
	p.pkg.Build.Migrations = pkg.Build.Migrations
	p.pkg.Build.RegistryOverrides = pkg.Build.RegistryOverrides
	p.pkg.Build.Differential = pkg.Build.Differential
	p.pkg.Build.DifferentialPackageVersion = pkg.Build.DifferentialPackageVersion
	p.pkg.Build.DifferentialMissing = pkg.Build.DifferentialMissing
	p.pkg.Build.Flavor = pkg.Build.Flavor
	p.pkg.Build.Signed = pkg.Build.Signed
	p.pkg.Build.ProvenanceFiles = pkg.Build.ProvenanceFiles
	p.pkg.Build.VersionRequirements = p.pkg.Build.VersionRequirements[:0]
	for _, requirement := range pkg.Build.VersionRequirements {
		p.pkg.Build.VersionRequirements = append(p.pkg.Build.VersionRequirements, internalTypes.VersionRequirement{
			Version: requirement.Version,
			Reason:  requirement.Reason,
		})
	}
}

// SetSignedProvenance marks the package as signed and records a generated provenance file.
func (p *PackageDefinition) SetSignedProvenance(file, version, reason string) {
	signed := true
	p.pkg.Build.Signed = &signed
	foundFile := false
	for _, existing := range p.pkg.Build.ProvenanceFiles {
		if existing == file {
			foundFile = true
			break
		}
	}
	if !foundFile {
		p.pkg.Build.ProvenanceFiles = append(p.pkg.Build.ProvenanceFiles, file)
	}
	for _, existing := range p.pkg.Build.VersionRequirements {
		if existing.Version == version && existing.Reason == reason {
			return
		}
	}
	p.pkg.Build.VersionRequirements = append(p.pkg.Build.VersionRequirements, internalTypes.VersionRequirement{
		Version: version,
		Reason:  reason,
	})
}
