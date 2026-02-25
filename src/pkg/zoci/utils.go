// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a reference for the given metadata.
func ReferenceFromMetadata(registryLocation string, pkg v1alpha1.ZarfPackage) (registry.Reference, error) {
	return ReferenceFromMetadataWithOptions(registryLocation, pkg, ReferenceFromMetadataOptions{})
}

// ReferenceFromMetadataOptions provides extensible options
// There may be a need for other OCI reference overwrite behaviors
type ReferenceFromMetadataOptions struct {
	Tag string
}

// ReferenceFromMetadataWithOptions returns a reference for the given metadata with optional overrides
func ReferenceFromMetadataWithOptions(registryLocation string, pkg v1alpha1.ZarfPackage, opts ReferenceFromMetadataOptions) (registry.Reference, error) {
	// Explicit requirement for version in order to publish
	if len(pkg.Metadata.Version) == 0 {
		return registry.Reference{}, errors.New("version is required for publishing")
	}
	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	// tag overwrite optionality
	tag := pkg.Metadata.Version
	if opts.Tag != "" {
		tag = opts.Tag
	}

	raw := fmt.Sprintf("%s%s:%s", registryLocation, pkg.Metadata.Name, tag)
	// this could be an option as well
	if pkg.Build.Flavor != "" {
		raw = fmt.Sprintf("%s-%s", raw, pkg.Build.Flavor)
	}

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("failed to parse %s: %w", raw, err)
	}
	return ref, nil
}

// GetInitPackageURL returns the URL for the init package for the given version.
func GetInitPackageURL(version string) string {
	return fmt.Sprintf("ghcr.io/zarf-dev/packages/init:%s", version)
}
