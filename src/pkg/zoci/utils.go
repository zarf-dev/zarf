// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oras-project/oras-go/v3/registry/remote/properties"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
)

// NormalizeOCISource adds the OCI scheme to valid registry references.
func NormalizeOCISource(source string) string {
	if helpers.IsOCIURL(source) || strings.Contains(source, "://") {
		return source
	}
	if strings.HasSuffix(source, ".tar.zst") || strings.HasSuffix(source, ".tar") || strings.Contains(source, ".part000") {
		return source
	}

	if _, err := properties.NewReference(source); err == nil {
		return helpers.OCIURLPrefix + source
	}
	return source
}

// ReferenceFromMetadata returns a reference for the given metadata.
func ReferenceFromMetadata(registryLocation string, pkg api.Package) (properties.Reference, error) {
	return ReferenceFromMetadataWithOptions(registryLocation, pkg, ReferenceFromMetadataOptions{})
}

// ReferenceFromMetadataOptions provides extensible options
type ReferenceFromMetadataOptions struct {
	// Tag specifies the OCI reference to use instead of package.metadata.version
	Tag string
}

// ReferenceFromMetadataWithOptions returns a reference for the given metadata with optional overrides
func ReferenceFromMetadataWithOptions(registryLocation string, pkg api.Package, opts ReferenceFromMetadataOptions) (properties.Reference, error) {
	// Explicit requirement for version in order to publish
	if len(pkg.Metadata.Version) == 0 {
		return properties.Reference{}, errors.New("version is required for publishing")
	}
	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	// Use the explicit tag if provided
	// do not include flavor if provided
	tag := pkg.Metadata.Version
	if opts.Tag != "" {
		tag = opts.Tag
	} else {
		if pkg.Build.Flavor != "" {
			tag = fmt.Sprintf("%s-%s", tag, pkg.Build.Flavor)
		}
	}

	raw := fmt.Sprintf("%s%s:%s", registryLocation, pkg.Metadata.Name, tag)

	ref, err := properties.NewReference(raw)
	if err != nil {
		return properties.Reference{}, fmt.Errorf("failed to parse %s: %w", raw, err)
	}
	return ref, nil
}
