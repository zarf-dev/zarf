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

// VersionRequirement specifies a minimum Zarf version needed and the reason for the requirement.
type VersionRequirement struct {
	Version string
	Reason  string
}

// BuildData contains version-neutral build metadata recorded during package assembly.
type BuildData struct {
	Hostname            string
	User                string
	Architecture        string
	Timestamp           string
	Version             string
	RegistryOverrides   map[string]string
	Flavor              string
	Signed              *bool
	VersionRequirements []VersionRequirement
	ProvenanceFiles     []string
	AggregateChecksum   string
}

// PackageDefinition is a concrete package source backed by the generic package representation.
type PackageDefinition struct {
	pkg internaltypes.Package
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

// RemoveImages removes images and image archives from every component.
func (p *PackageDefinition) RemoveImages() {
	for i := range p.pkg.Components {
		p.pkg.Components[i].Images = nil
		p.pkg.Components[i].ImageArchives = nil
	}
}

// RemoveRepositories removes git repositories from every component.
func (p *PackageDefinition) RemoveRepositories() {
	for i := range p.pkg.Components {
		p.pkg.Components[i].Repositories = nil
	}
}

// RetainComponents retains the components at the given indices, in order.
func (p *PackageDefinition) RetainComponents(indices []int) error {
	components := make([]internaltypes.Component, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(p.pkg.Components) {
			return fmt.Errorf("component index %d out of range", idx)
		}
		components = append(components, p.pkg.Components[idx])
	}
	p.pkg.Components = components
	return nil
}

// OverrideNamespace overrides component namespaces when the definition permits it.
func (p *PackageDefinition) OverrideNamespace(namespace string) error {
	if !p.pkg.Metadata.PreventNamespaceOverride {
		return fmt.Errorf("cannot override package namespace, metadata.allowNamespaceOverride is false")
	}
	if p.pkg.Kind != string(v1alpha1.ZarfPackageConfig) {
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
	for i := range p.pkg.Components {
		component := &p.pkg.Components[i]
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

func (p PackageDefinition) uniqueNamespaces() []string {
	seen := map[string]struct{}{}
	for _, component := range p.pkg.Components {
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
	for i := range p.pkg.Components {
		component := &p.pkg.Components[i]
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

// SetMetadataVersion sets the package metadata version.
func (p *PackageDefinition) SetMetadataVersion(version string) {
	p.pkg.Metadata.Version = version
}

// SetMetadataArchitecture sets the package metadata architecture.
func (p *PackageDefinition) SetMetadataArchitecture(architecture string) {
	p.pkg.Metadata.Architecture = architecture
}

// SetBuildData records the version-neutral build metadata generated during package assembly.
func (p *PackageDefinition) SetBuildData(buildData BuildData) {
	p.pkg.Build.Hostname = buildData.Hostname
	p.pkg.Build.User = buildData.User
	p.pkg.Build.Architecture = buildData.Architecture
	p.pkg.Build.Timestamp = buildData.Timestamp
	p.pkg.Build.Version = buildData.Version
	p.pkg.Build.RegistryOverrides = maps.Clone(buildData.RegistryOverrides)
	p.pkg.Build.Flavor = buildData.Flavor
	p.pkg.Build.Signed = cloneBool(buildData.Signed)
	p.pkg.Build.ProvenanceFiles = slices.Clone(buildData.ProvenanceFiles)
	p.pkg.Build.VersionRequirements = versionRequirementsToInternal(buildData.VersionRequirements)
	p.pkg.Metadata.AggregateChecksum = buildData.AggregateChecksum
	p.pkg.Build.AggregateChecksum = buildData.AggregateChecksum
}

// SetDifferentialBuild records the base package version for a differential build.
func (p *PackageDefinition) SetDifferentialBuild(packageVersion string) {
	p.pkg.Build.Differential = true
	p.pkg.Build.DifferentialPackageVersion = packageVersion
}

// SetBuildSigned sets whether the package build is signed.
func (p *PackageDefinition) SetBuildSigned(signed bool) {
	p.pkg.Build.Signed = &signed
}

// AddProvenanceFile records a package build provenance file once.
func (p *PackageDefinition) AddProvenanceFile(file string) {
	if slices.Contains(p.pkg.Build.ProvenanceFiles, file) {
		return
	}
	p.pkg.Build.ProvenanceFiles = append(p.pkg.Build.ProvenanceFiles, file)
}

// AddVersionRequirement records a package build version requirement once.
func (p *PackageDefinition) AddVersionRequirement(requirement VersionRequirement) {
	if slices.ContainsFunc(p.pkg.Build.VersionRequirements, func(existing internaltypes.VersionRequirement) bool {
		return existing.Version == requirement.Version && existing.Reason == requirement.Reason
	}) {
		return
	}
	p.pkg.Build.VersionRequirements = append(p.pkg.Build.VersionRequirements, internaltypes.VersionRequirement{
		Version: requirement.Version,
		Reason:  requirement.Reason,
	})
}

func versionRequirementsToInternal(requirements []VersionRequirement) []internaltypes.VersionRequirement {
	if len(requirements) == 0 {
		return nil
	}
	converted := make([]internaltypes.VersionRequirement, len(requirements))
	for i, requirement := range requirements {
		converted[i] = internaltypes.VersionRequirement{
			Version: requirement.Version,
			Reason:  requirement.Reason,
		}
	}
	return converted
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
