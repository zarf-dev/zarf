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
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// PackageAccessor is the read contract for a package source, exposing a per-version definition.
type PackageAccessor interface {
	AsV1alpha1() v1alpha1.ZarfPackage
	AsV1beta1() v1beta1.Package
}

// PackageDefinition is a concrete package source backed by the internal generic package representation.
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

// FilterComponents applies a component filter to the package definition.
func (p *PackageDefinition) FilterComponents(filter filters.ComponentFilterStrategy) error {
	if filter == nil {
		filter = filters.Empty()
	}
	indices, err := filter.Apply(p.filterView())
	if err != nil {
		return err
	}
	components := make([]internalTypes.Component, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(p.pkg.Components) {
			return fmt.Errorf("filter returned index %d out of range", idx)
		}
		components = append(components, p.pkg.Components[idx])
	}
	p.pkg.Components = components
	return nil
}

// OverrideNamespace overrides component namespaces when the definition permits it.
func (p *PackageDefinition) OverrideNamespace(namespace string) error {
	if !p.allowsNamespaceOverride() {
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

func (p PackageDefinition) allowsNamespaceOverride() bool {
	if p.pkg.Metadata.AllowNamespaceOverride != nil {
		return *p.pkg.Metadata.AllowNamespaceOverride
	}
	return !p.pkg.Metadata.PreventNamespaceOverride
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
	namespaces := make([]string, 0, len(seen))
	for namespace := range seen {
		namespaces = append(namespaces, namespace)
	}
	return namespaces
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

func overrideActionSetWaitNamespaces(set *internalTypes.ComponentActionSet, original, target string) {
	overrideActionWaitNamespaces(set.Before, original, target)
	overrideActionWaitNamespaces(set.After, original, target)
	overrideActionWaitNamespaces(set.OnSuccess, original, target)
	overrideActionWaitNamespaces(set.OnFailure, original, target)
}

func overrideActionWaitNamespaces(actions []internalTypes.ComponentAction, original, target string) {
	for i := range actions {
		if actions[i].Wait != nil && actions[i].Wait.Cluster != nil && actions[i].Wait.Cluster.Namespace == original {
			actions[i].Wait.Cluster.Namespace = target
		}
	}
}

func (p PackageDefinition) filterView() filters.PackageView {
	components := make([]filters.ComponentView, 0, len(p.pkg.Components))
	for _, component := range p.pkg.Components {
		components = append(components, filters.ComponentView{
			Name:        component.Name,
			Description: component.Description,
			Optional:    component.Optional,
			Default:     component.Default,
			Group:       component.Group,
			OnlyLocalOS: component.Target.OS,
		})
	}
	return filters.PackageView{Components: components}
}

// SetAggregateChecksum records the generated checksums.txt aggregate checksum in every API location.
func (p *PackageDefinition) SetAggregateChecksum(checksum string) {
	p.pkg.Metadata.AggregateChecksum = checksum
	p.pkg.Build.AggregateChecksum = checksum
}

// SetMetadataVersion sets the package metadata version.
func (p *PackageDefinition) SetMetadataVersion(version string) {
	p.pkg.Metadata.Version = version
}

// SetBuildHostname sets the host that created the package.
func (p *PackageDefinition) SetBuildHostname(hostname string) {
	p.pkg.Build.Hostname = hostname
}

// SetBuildUser sets the user that created the package.
func (p *PackageDefinition) SetBuildUser(user string) {
	p.pkg.Build.User = user
}

// SetBuildArchitecture sets the package build architecture.
func (p *PackageDefinition) SetBuildArchitecture(architecture string) {
	p.pkg.Build.Architecture = architecture
}

// SetBuildTimestamp sets the package build timestamp.
func (p *PackageDefinition) SetBuildTimestamp(timestamp string) {
	p.pkg.Build.Timestamp = timestamp
}

// SetBuildVersion sets the Zarf CLI version used to build the package.
func (p *PackageDefinition) SetBuildVersion(version string) {
	p.pkg.Build.Version = version
}

// SetBuildMigrations sets the package build migrations.
func (p *PackageDefinition) SetBuildMigrations(migrations []string) {
	p.pkg.Build.Migrations = slices.Clone(migrations)
}

// SetBuildRegistryOverrides sets the package build registry overrides.
func (p *PackageDefinition) SetBuildRegistryOverrides(registryOverrides map[string]string) {
	p.pkg.Build.RegistryOverrides = maps.Clone(registryOverrides)
}

// SetBuildDifferential sets the package differential build data.
func (p *PackageDefinition) SetBuildDifferential(differential bool, packageVersion string, missing []string) {
	p.pkg.Build.Differential = differential
	p.pkg.Build.DifferentialPackageVersion = packageVersion
	p.pkg.Build.DifferentialMissing = slices.Clone(missing)
}

// SetBuildFlavor sets the package build flavor.
func (p *PackageDefinition) SetBuildFlavor(flavor string) {
	p.pkg.Build.Flavor = flavor
}

// SetBuildSigned sets whether the package build is signed.
func (p *PackageDefinition) SetBuildSigned(signed bool) {
	p.pkg.Build.Signed = &signed
}

// SetProvenanceFiles sets the package build provenance files.
func (p *PackageDefinition) SetProvenanceFiles(files []string) {
	p.pkg.Build.ProvenanceFiles = slices.Clone(files)
}

// AddProvenanceFile records a package build provenance file once.
func (p *PackageDefinition) AddProvenanceFile(file string) {
	for _, existing := range p.pkg.Build.ProvenanceFiles {
		if existing == file {
			return
		}
	}
	p.pkg.Build.ProvenanceFiles = append(p.pkg.Build.ProvenanceFiles, file)
}

// AddVersionRequirement records a package build version requirement once.
func (p *PackageDefinition) AddVersionRequirement(requirement v1alpha1.VersionRequirement) {
	for _, existing := range p.pkg.Build.VersionRequirements {
		if existing.Version == requirement.Version && existing.Reason == requirement.Reason {
			return
		}
	}
	p.pkg.Build.VersionRequirements = append(p.pkg.Build.VersionRequirements, internalTypes.VersionRequirement{
		Version: requirement.Version,
		Reason:  requirement.Reason,
	})
}
