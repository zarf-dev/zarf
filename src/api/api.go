// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package api defines the version-neutral Zarf package model.
package api

import (
	"fmt"
	"maps"
	"slices"
)

// PackageKind identifies the kind of a Zarf package.
type PackageKind string

const (
	// ZarfInitConfig is the package kind used during zarf init.
	ZarfInitConfig PackageKind = "ZarfInitConfig"
	// ZarfPackageConfig is the default package kind.
	ZarfPackageConfig PackageKind = "ZarfPackageConfig"
)

// SetName updates the package metadata name.
func (p *Package) SetName(name string) {
	p.Metadata.Name = name
}

// SetAnnotations updates the package metadata annotations.
func (p *Package) SetAnnotations(annotations map[string]string) {
	p.Metadata.Annotations = maps.Clone(annotations)
}

// RemoveImages removes images and image archives from every component.
func (p *Package) RemoveImages() {
	for i := range p.Components {
		p.Components[i].Images, p.Components[i].ImageArchives = nil, nil
	}
}

// RemoveRepositories removes git repositories from every component.
func (p *Package) RemoveRepositories() {
	for i := range p.Components {
		p.Components[i].Repositories = nil
	}
}

// RetainComponents retains the components at the given indices, in order.
func (p *Package) RetainComponents(indices []int) error {
	components := make([]Component, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(p.Components) {
			return fmt.Errorf("component index %d out of range", idx)
		}
		components = append(components, p.Components[idx])
	}
	p.Components = components
	return nil
}

// OverrideNamespace overrides component namespaces when the package permits it.
func (p *Package) OverrideNamespace(namespace string) error {
	if p.Metadata.PreventNamespaceOverride {
		return fmt.Errorf("package explicitly prevents namespace overrides")
	}
	if p.Kind == ZarfInitConfig {
		return fmt.Errorf("package kind is not a ZarfPackageConfig, cannot override namespace")
	}
	namespaces := p.uniqueNamespaces()
	if len(namespaces) > 1 {
		return fmt.Errorf("package contains %d unique namespaces, cannot override namespace", len(namespaces))
	}
	original := ""
	if len(namespaces) == 1 {
		original = namespaces[0]
	}
	for i := range p.Components {
		p.Components[i].overrideNamespaces(original, namespace)
	}
	return nil
}

// SetChartNamespace sets the namespace for charts matching the component and chart names.
func (p *Package) SetChartNamespace(componentName, chartName, namespace string) {
	for i := range p.Components {
		if p.Components[i].Name != componentName {
			continue
		}
		for j := range p.Components[i].Charts {
			if p.Components[i].Charts[j].Name == chartName {
				p.Components[i].Charts[j].Namespace = namespace
			}
		}
	}
}

func (p Package) uniqueNamespaces() []string {
	seen := map[string]struct{}{}
	for _, component := range p.Components {
		for _, chart := range component.Charts {
			seen[chart.Namespace] = struct{}{}
		}
		for _, manifest := range component.Manifests {
			seen[manifest.Namespace] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(seen))
}

func (c *Component) overrideNamespaces(original, target string) {
	for i := range c.Charts {
		if c.Charts[i].Namespace == original {
			c.Charts[i].Namespace = target
		}
	}
	for i := range c.Manifests {
		if c.Manifests[i].Namespace == original {
			c.Manifests[i].Namespace = target
		}
	}
	for _, set := range []*ActionSet{&c.Actions.OnCreate, &c.Actions.OnDeploy, &c.Actions.OnRemove} {
		for _, actions := range []*[]Action{&set.Before, &set.After, &set.OnSuccess, &set.OnFailure} {
			for i := range *actions {
				wait := (*actions)[i].Wait
				if wait != nil && wait.Cluster != nil && wait.Cluster.Namespace == original {
					wait.Cluster.Namespace = target
				}
			}
		}
	}
}

// SetBuildSigned records whether the package build is signed.
func (p *Package) SetBuildSigned(signed bool) {
	p.Build.Signed = &signed
}

// AddProvenanceFile records a provenance file once.
func (p *Package) AddProvenanceFile(file string) {
	if !slices.Contains(p.Build.ProvenanceFiles, file) {
		p.Build.ProvenanceFiles = append(p.Build.ProvenanceFiles, file)
	}
}

// AddVersionRequirement records a version requirement once.
func (p *Package) AddVersionRequirement(requirement VersionRequirement) {
	if !slices.ContainsFunc(p.Build.VersionRequirements, func(existing VersionRequirement) bool {
		return existing == requirement
	}) {
		p.Build.VersionRequirements = append(p.Build.VersionRequirements, requirement)
	}
}
