// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package api defines the version-neutral Zarf package model.
package api

import (
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// PackageKind identifies the kind of a Zarf package.
type PackageKind string

const (
	// ZarfInitConfig is the package kind used during zarf init.
	ZarfInitConfig PackageKind = "ZarfInitConfig"
	// ZarfPackageConfig is the default package kind.
	ZarfPackageConfig PackageKind = "ZarfPackageConfig"
)

// BuildTimestampFormat is the timestamp format used for package build metadata.
const BuildTimestampFormat = time.RFC1123Z

// GetAPIVersion returns the package API version, treating the legacy omitted value as v1alpha1.
func (p Package) GetAPIVersion() string {
	if p.APIVersion == "" {
		return v1alpha1.APIVersion
	}
	return p.APIVersion
}

// IsSBOMAble reports whether this package contains content that can have an SBOM.
func (p Package) IsSBOMAble() bool {
	for _, component := range p.Components {
		if len(component.ImageArchives) > 0 || len(component.Images) > 0 || len(component.Files) > 0 || len(component.DataInjections) > 0 {
			return true
		}
	}
	return false
}

// HasImages reports whether a package contains images or image archives.
func (p Package) HasImages() bool {
	for _, component := range p.Components {
		if len(component.Images) > 0 || len(component.ImageArchives) > 0 {
			return true
		}
	}
	return false
}

// IsInitConfig reports whether this is a Zarf init package.
func (p Package) IsInitConfig() bool {
	return p.Kind == ZarfInitConfig
}

// RequiresCluster reports whether this component requires a cluster connection.
func (c Component) RequiresCluster() bool {
	return len(c.Images) > 0 || len(c.ImageArchives) > 0 || len(c.Charts) > 0 ||
		len(c.Manifests) > 0 || len(c.Repositories) > 0 || len(c.DataInjections) > 0 ||
		len(c.HealthChecks) > 0
}

// IsRequired reports whether this component is required.
func (c Component) IsRequired() bool {
	return !c.Optional
}

// GetImages returns all images specified by this component, including image archives.
func (c Component) GetImages() []string {
	images := make([]string, 0, len(c.Images))
	for _, image := range c.Images {
		images = append(images, image.Name)
	}
	for _, archive := range c.ImageArchives {
		images = append(images, archive.Images...)
	}
	return images
}

// IsTemplate reports whether this file should be rendered as a Go template.
func (f File) IsTemplate() bool {
	return f.EnableTemplating
}

// ShouldRunSchemaValidation reports whether Helm values schema validation is enabled.
func (c Chart) ShouldRunSchemaValidation() bool {
	return !c.SkipSchemaValidation
}

// GetServerSideApply returns the configured apply strategy, defaulting to auto.
func (c Chart) GetServerSideApply() string {
	if c.ServerSideApply == "" {
		return "auto"
	}
	return c.ServerSideApply
}

// SourceURL returns the chart source URL, if the chart is remotely sourced.
func (c Chart) SourceURL() string {
	switch {
	case c.HelmRepository != nil:
		return c.HelmRepository.URL
	case c.Git != nil:
		return c.Git.URL
	case c.OCI != nil:
		return c.OCI.URL
	default:
		return ""
	}
}

// LocalPath returns the source path for a local chart.
func (c Chart) LocalPath() string {
	if c.Local == nil {
		return ""
	}
	return c.Local.Path
}

// RepositoryName returns the named chart in a Helm repository, if set.
func (c Chart) RepositoryName() string {
	if c.HelmRepository == nil {
		return ""
	}
	return c.HelmRepository.Name
}

// GitPath returns the chart path within a Git source, if set.
func (c Chart) GitPath() string {
	if c.Git == nil {
		return ""
	}
	return c.Git.Path
}

// GetServerSideApply returns the configured apply strategy, defaulting to auto.
func (m Manifest) GetServerSideApply() string {
	if m.ServerSideApply == "" {
		return "auto"
	}
	return m.ServerSideApply
}

// IsTemplate reports whether this manifest should be rendered as a Go template.
func (m Manifest) IsTemplate() bool {
	return m.EnableTemplating
}

// RemoveImages removes images and image archives from every component.
func (p *Package) RemoveImages() {
	for i := range p.Components {
		p.Components[i].Images = nil
		p.Components[i].ImageArchives = nil
	}
}

// RemoveRepositories removes git repositories from every component.
func (p *Package) RemoveRepositories() {
	for i := range p.Components {
		p.Components[i].Repositories = nil
	}
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
