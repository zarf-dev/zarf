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
func ReferenceFromMetadata(registryLocation string, pkg v1alpha1.ZarfPackage) (string, error) {
	if len(pkg.Metadata.Version) == 0 {
		return "", errors.New("version is required for publishing")
	}
	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	raw := fmt.Sprintf("%s%s:%s", registryLocation, pkg.Metadata.Name, pkg.Metadata.Version)
	if pkg.Build.Flavor != "" {
		raw = fmt.Sprintf("%s-%s", raw, pkg.Build.Flavor)
	}

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", raw, err)
	}
	return ref.String(), nil
}

// GetInitPackageURL returns the URL for the init package for the given version.
func GetInitPackageURL(version string) string {
	return fmt.Sprintf("ghcr.io/zarf-dev/packages/init:%s", version)
}
