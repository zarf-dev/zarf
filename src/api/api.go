// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package api supplies helpers for working generically with the different API versions
package api

import (
	"fmt"
	"maps"
	"slices"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internaltypes "github.com/zarf-dev/zarf/src/internal/api/types"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
)

// PackageAccessor is the read contract for a package source, exposing a per-version definition.
type PackageAccessor interface {
	AsV1alpha1() v1alpha1.ZarfPackage
	AsV1beta1() v1beta1.Package
}

// PackageDefinition is a concrete package source backed by the generic package representation.
type PackageDefinition struct {
	// Pkg is the canonical representation used by Zarf's internal package-processing code.
	// Intentionally unavailable to SDK consumers who can either use targeted setters or get the package as a specific API version
	Pkg internaltypes.Package
}

var _ PackageAccessor = PackageDefinition{}

// NewPackageDefinitionFromV1alpha1 creates a PackageDefinition from a v1alpha1 package definition.
func NewPackageDefinitionFromV1alpha1(pkg v1alpha1.ZarfPackage) PackageDefinition {
	return PackageDefinition{Pkg: internalv1alpha1.ConvertToGeneric(pkg)}
}

// NewPackageDefinitionFromV1beta1 creates a PackageDefinition from a v1beta1 package definition.
func NewPackageDefinitionFromV1beta1(pkg v1beta1.Package) PackageDefinition {
	return PackageDefinition{Pkg: internalv1beta1.ConvertToGeneric(pkg)}
}

// AsV1alpha1 returns the package definition as a v1alpha1 ZarfPackage.
func (p PackageDefinition) AsV1alpha1() v1alpha1.ZarfPackage {
	return internalv1alpha1.ConvertFromGeneric(p.Pkg)
}

// AsV1beta1 returns the package definition as a v1beta1 Package.
func (p PackageDefinition) AsV1beta1() v1beta1.Package {
	return internalv1beta1.ConvertFromGeneric(p.Pkg)
}

// OriginalAPIVersion returns the apiVersion the package was authored in before any conversion.
func (p PackageDefinition) OriginalAPIVersion() string {
	return p.Pkg.Build.OriginalAPIVersion
}

// SetName sets the package metadata name.
func (p *PackageDefinition) SetName(name string) {
	p.Pkg.Metadata.Name = name
}

// SetAnnotations sets the package metadata annotations.
func (p *PackageDefinition) SetAnnotations(annotations map[string]string) {
	p.Pkg.Metadata.Annotations = annotations
}

// RemoveImages removes images and image archives from every component.
func (p *PackageDefinition) RemoveImages() {
	for i := range p.Pkg.Components {
		p.Pkg.Components[i].Images = nil
		p.Pkg.Components[i].ImageArchives = nil
	}
}

// RemoveRepositories removes git repositories from every component.
func (p *PackageDefinition) RemoveRepositories() {
	for i := range p.Pkg.Components {
		p.Pkg.Components[i].Repositories = nil
	}
}

// OverrideNamespace overrides component namespaces when the definition permits it.
func (p *PackageDefinition) OverrideNamespace(namespace string) error {
	if !p.allowsNamespaceOverride() {
		return fmt.Errorf("cannot override package namespace, metadata.allowNamespaceOverride is false")
	}
	if p.Pkg.Kind != string(v1alpha1.ZarfPackageConfig) {
		return fmt.Errorf("package kind is not a ZarfPackageConfig, cannot override namespace")
	}
	namespaces := p.uniqueNamespaces()
	if len(namespaces) > 1 {
		return fmt.Errorf("package contains %d unique namespaces, cannot override namespace", len(namespaces))
	}
	originalNamespace := ""
	if len(namespaces) == 1 {
		originalNamespace = namespaces[0]
	}
	p.overrideComponentNamespaces(originalNamespace, namespace)
	return nil
}

// SetChartNamespace sets the namespace for charts matching the given component and chart name.
func (p *PackageDefinition) SetChartNamespace(componentName, chartName, namespace string) {
	for i := range p.Pkg.Components {
		component := &p.Pkg.Components[i]
		if component.Name != componentName {
			continue
		}
		for j := range component.Charts {
			if component.Charts[j].Name == chartName {
				component.Charts[j].Namespace = namespace
			}
		}
	}
}

func (p PackageDefinition) allowsNamespaceOverride() bool {
	if p.Pkg.Metadata.AllowNamespaceOverride != nil {
		return *p.Pkg.Metadata.AllowNamespaceOverride
	}
	return !p.Pkg.Metadata.PreventNamespaceOverride
}

func (p PackageDefinition) uniqueNamespaces() []string {
	seen := map[string]struct{}{}
	for _, component := range p.Pkg.Components {
		for _, chart := range component.Charts {
			seen[chart.Namespace] = struct{}{}
		}
		for _, manifest := range component.Manifests {
			seen[manifest.Namespace] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(seen))
}

func (p *PackageDefinition) overrideComponentNamespaces(original, target string) {
	for i := range p.Pkg.Components {
		component := &p.Pkg.Components[i]
		for j := range component.Charts {
			if component.Charts[j].Namespace == original {
				component.Charts[j].Namespace = target
			}
		}
		for j := range component.Manifests {
			if component.Manifests[j].Namespace == original {
				component.Manifests[j].Namespace = target
			}
		}
		overrideActionSetWaitNamespaces(&component.Actions.OnCreate, original, target)
		overrideActionSetWaitNamespaces(&component.Actions.OnDeploy, original, target)
		overrideActionSetWaitNamespaces(&component.Actions.OnRemove, original, target)
	}
}

func overrideActionSetWaitNamespaces(set *internaltypes.ComponentActionSet, original, target string) {
	overrideActionWaitNamespaces(set.Before, original, target)
	overrideActionWaitNamespaces(set.After, original, target)
	overrideActionWaitNamespaces(set.OnSuccess, original, target)
	overrideActionWaitNamespaces(set.OnFailure, original, target)
}

func overrideActionWaitNamespaces(actions []internaltypes.ComponentAction, original, target string) {
	for i := range actions {
		if actions[i].Wait != nil && actions[i].Wait.Cluster != nil && actions[i].Wait.Cluster.Namespace == original {
			actions[i].Wait.Cluster.Namespace = target
		}
	}
}

// SetAggregateChecksum records the generated checksums.txt aggregate checksum in every API location.
func (p *PackageDefinition) SetAggregateChecksum(checksum string) {
	p.Pkg.Metadata.AggregateChecksum = checksum
	p.Pkg.Build.AggregateChecksum = checksum
}

// SetMetadataVersion sets the package metadata version.
func (p *PackageDefinition) SetMetadataVersion(version string) {
	p.Pkg.Metadata.Version = version
}

// SetMetadataArchitecture sets the package metadata architecture.
func (p *PackageDefinition) SetMetadataArchitecture(architecture string) {
	p.Pkg.Metadata.Architecture = architecture
}

// SetBuildHostname sets the host that created the package.
func (p *PackageDefinition) SetBuildHostname(hostname string) {
	p.Pkg.Build.Hostname = hostname
}

// SetBuildUser sets the user that created the package.
func (p *PackageDefinition) SetBuildUser(user string) {
	p.Pkg.Build.User = user
}

// SetBuildArchitecture sets the package build architecture.
func (p *PackageDefinition) SetBuildArchitecture(architecture string) {
	p.Pkg.Build.Architecture = architecture
}

// SetBuildTimestamp sets the package build timestamp.
func (p *PackageDefinition) SetBuildTimestamp(timestamp string) {
	p.Pkg.Build.Timestamp = timestamp
}

// SetBuildVersion sets the Zarf CLI version used to build the package.
func (p *PackageDefinition) SetBuildVersion(version string) {
	p.Pkg.Build.Version = version
}

// SetBuildMigrations sets the package build migrations.
func (p *PackageDefinition) SetBuildMigrations(migrations []string) {
	p.Pkg.Build.Migrations = slices.Clone(migrations)
}

// SetBuildRegistryOverrides sets the package build registry overrides.
func (p *PackageDefinition) SetBuildRegistryOverrides(registryOverrides map[string]string) {
	p.Pkg.Build.RegistryOverrides = maps.Clone(registryOverrides)
}

// SetBuildDifferential sets the package differential build data.
func (p *PackageDefinition) SetBuildDifferential(differential bool, packageVersion string, missing []string) {
	p.Pkg.Build.Differential = differential
	p.Pkg.Build.DifferentialPackageVersion = packageVersion
	p.Pkg.Build.DifferentialMissing = slices.Clone(missing)
}

// SetBuildFlavor sets the package build flavor.
func (p *PackageDefinition) SetBuildFlavor(flavor string) {
	p.Pkg.Build.Flavor = flavor
}

// SetBuildSigned sets whether the package build is signed.
func (p *PackageDefinition) SetBuildSigned(signed bool) {
	p.Pkg.Build.Signed = &signed
}

// SetProvenanceFiles sets the package build provenance files.
func (p *PackageDefinition) SetProvenanceFiles(files []string) {
	p.Pkg.Build.ProvenanceFiles = slices.Clone(files)
}

// AddProvenanceFile records a package build provenance file once.
func (p *PackageDefinition) AddProvenanceFile(file string) {
	if slices.Contains(p.Pkg.Build.ProvenanceFiles, file) {
		return
	}
	p.Pkg.Build.ProvenanceFiles = append(p.Pkg.Build.ProvenanceFiles, file)
}

// SetBuildVersionRequirements sets the package build version requirements.
func (p *PackageDefinition) SetBuildVersionRequirements(requirements []v1alpha1.VersionRequirement) {
	if len(requirements) == 0 {
		p.Pkg.Build.VersionRequirements = nil
		return
	}
	p.Pkg.Build.VersionRequirements = make([]internaltypes.VersionRequirement, len(requirements))
	for i, requirement := range requirements {
		p.Pkg.Build.VersionRequirements[i] = internaltypes.VersionRequirement{
			Version: requirement.Version,
			Reason:  requirement.Reason,
		}
	}
}

// AddVersionRequirement records a package build version requirement once.
func (p *PackageDefinition) AddVersionRequirement(requirement v1alpha1.VersionRequirement) {
	if slices.ContainsFunc(p.Pkg.Build.VersionRequirements, func(existing internaltypes.VersionRequirement) bool {
		return existing.Version == requirement.Version && existing.Reason == requirement.Reason
	}) {
		return
	}
	p.Pkg.Build.VersionRequirements = append(p.Pkg.Build.VersionRequirements, internaltypes.VersionRequirement{
		Version: requirement.Version,
		Reason:  requirement.Reason,
	})
}
