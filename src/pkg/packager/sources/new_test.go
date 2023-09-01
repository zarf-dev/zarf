// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package sources

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

var ocip *OCISource
var urlp *URLSource
var tarballp *TarballSource
var partialp *PartialTarballSource
var packagep *types.PackageSource

type source struct {
	pkgSrc  string
	srcType string
	source  types.PackageSource
}

var sources = []source{
	{pkgSrc: "oci://ghcr.io/defenseunicorns/packages/init:1.0.0-amd64", srcType: "oci", source: ocip},
	{pkgSrc: "sget://github.com/defenseunicorns/zarf-hello-world:x86", srcType: "sget", source: urlp},
	{pkgSrc: "sget://defenseunicorns/zarf-hello-world:x86_64", srcType: "sget", source: urlp},
	{pkgSrc: "https://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst", srcType: "https", source: urlp},
	{pkgSrc: "http://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst", srcType: "http", source: urlp},
	{pkgSrc: "zarf-init-amd64-v1.0.0.tar.zst", srcType: "tarball", source: tarballp},
	{pkgSrc: "zarf-package-manifests-amd64-v1.0.0.tar", srcType: "tarball", source: tarballp},
	{pkgSrc: "zarf-package-manifests-amd64-v1.0.0.tar.zst", srcType: "tarball", source: tarballp},
	{pkgSrc: "some-dir/.part000", srcType: "partial", source: partialp},
}

func Test_identifySourceType(t *testing.T) {
	for _, source := range sources {
		actual := identifySourceType(source.pkgSrc)
		require.Equalf(t, source.srcType, actual, fmt.Sprintf("source: %s", source))
	}
}

func TestNew(t *testing.T) {
	for _, source := range sources {
		actual, err := New(&types.ZarfPackageOptions{PackageSource: source.pkgSrc}, "")
		require.NoError(t, err)
		require.IsType(t, source.source, actual)
		require.Implements(t, packagep, actual)
	}
}
