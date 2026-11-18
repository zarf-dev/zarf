// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"errors"
	"fmt"
	"strings"

	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	legacyerrdef "oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a registry reference for the given package metadata.
// FIXME: consider if I want this
//
// Deprecated: Use OCIReferenceFromMetadata, which returns an oci.Reference.
func ReferenceFromMetadata(registryLocation string, pkg api.Package) (registry.Reference, error) {
	return legacyReferenceResult(OCIReferenceFromMetadata(registryLocation, pkg))
}

// ReferenceFromMetadataWithOptions returns a registry reference with optional overrides.
//
// Deprecated: Use OCIReferenceFromMetadataWithOptions, which returns an oci.Reference.
func ReferenceFromMetadataWithOptions(registryLocation string, pkg api.Package, opts ReferenceFromMetadataOptions) (registry.Reference, error) {
	return legacyReferenceResult(OCIReferenceFromMetadataWithOptions(registryLocation, pkg, opts))
}

func legacyReferenceResult(ref oci.Reference, err error) (registry.Reference, error) {
	if err != nil {
		if errors.Is(err, errdef.ErrInvalidReference) {
			err = fmt.Errorf("%w: %w", legacyerrdef.ErrInvalidReference, err)
		}
		return registry.Reference{}, err
	}
	return registry.Reference{Registry: ref.Registry, Repository: ref.Repository, Reference: ref.GetReference()}, nil
}

// NormalizeOCISource adds the OCI scheme to valid registry references.
func NormalizeOCISource(source string) string {
	if helpers.IsOCIURL(source) || strings.Contains(source, "://") {
		return source
	}
	if strings.HasSuffix(source, ".tar.zst") || strings.HasSuffix(source, ".tar") || strings.Contains(source, ".part000") {
		return source
	}

	if _, err := oci.ParseReference(source); err == nil {
		return helpers.OCIURLPrefix + source
	}
	return source
}

// OCIReferenceFromMetadata returns a reference for the given metadata.
func OCIReferenceFromMetadata(registryLocation string, pkg api.Package) (oci.Reference, error) {
	return OCIReferenceFromMetadataWithOptions(registryLocation, pkg, ReferenceFromMetadataOptions{})
}

// ReferenceFromMetadataOptions provides extensible options
type ReferenceFromMetadataOptions struct {
	// Tag specifies the OCI reference to use instead of package.metadata.version
	Tag string
}

// OCIReferenceFromMetadataWithOptions returns a reference for the given metadata with optional overrides
func OCIReferenceFromMetadataWithOptions(registryLocation string, pkg api.Package, opts ReferenceFromMetadataOptions) (oci.Reference, error) {
	// Explicit requirement for version in order to publish
	if len(pkg.Metadata.Version) == 0 {
		return oci.Reference{}, errors.New("version is required for publishing")
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

	ref, err := oci.ParseReference(raw)
	if err != nil {
		return oci.Reference{}, fmt.Errorf("failed to parse %s: %w", raw, err)
	}
	return ref, nil
}
