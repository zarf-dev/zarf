// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

var ocip *OCIProvider
var urlp *URLProvider
var tarballp *TarballProvider
var partialp *PartialTarballProvider
var packagep *types.PackageProvider

type source struct {
	src      string
	srcType  string
	provider types.PackageProvider
}

var sources = []source{
	{src: "oci://ghcr.io/defenseunicorns/packages/init:1.0.0-amd64", srcType: "oci", provider: ocip},
	{src: "sget://github.com/defenseunicorns/zarf-hello-world:x86", srcType: "sget", provider: urlp},
	{src: "sget://defenseunicorns/zarf-hello-world:x86_64", srcType: "sget", provider: urlp},
	{src: "https://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst", srcType: "https", provider: urlp},
	{src: "http://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst", srcType: "http", provider: urlp},
	{src: "zarf-init-amd64-v1.0.0.tar.zst", srcType: "tarball", provider: tarballp},
	{src: "zarf-package-manifests-amd64-v1.0.0.tar", srcType: "tarball", provider: tarballp},
	{src: "zarf-package-manifests-amd64-v1.0.0.tar.zst", srcType: "tarball", provider: tarballp},
	{src: "some-dir/.part000", srcType: "partial", provider: partialp},
}

func Test_identifySourceType(t *testing.T) {
	for _, source := range sources {
		actual := identifySourceType(source.src)
		require.Equalf(t, source.srcType, actual, fmt.Sprintf("source: %s", source))
	}
}

func TestProviderFromSource(t *testing.T) {
	for _, source := range sources {
		actual, err := ProviderFromSource(&types.ZarfPackageOptions{PackagePath: source.src}, "")
		require.NoError(t, err)
		require.IsType(t, source.provider, actual)
		require.Implements(t, packagep, actual)
	}
}
