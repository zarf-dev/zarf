// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package ocizarf contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a reference for the given metadata.
func ReferenceFromMetadata(registryLocation string, metadata *types.ZarfMetadata, build *types.ZarfBuildData) (string, error) {
	ver := metadata.Version
	if len(ver) == 0 {
		return "", errors.New("version is required for publishing")
	}

	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	format := "%s%s:%s"
	raw := fmt.Sprintf(format, registryLocation, metadata.Name, ver)

	if build != nil && build.Flavor != "" {
		raw = fmt.Sprintf("%s-%s", raw, build.Flavor)
	}

	message.Debug("Raw OCI reference from metadata:", raw)

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", err
	}

	return ref.String(), nil
}

// GetInitPackageURL returns the URL for the init package for the given version.
func GetInitPackageURL(version string) string {
	return fmt.Sprintf("ghcr.io/defenseunicorns/packages/init:%s", version)
}
