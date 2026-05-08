// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"encoding/json"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	_ "modernc.org/sqlite"
)

func TestPushPackage(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	pkg := buildVirtualPackage(ctx, t)

	pkgLayout, err := layout.LoadFromTar(ctx, pkg.packagePath, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)

	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := zoci.NewRemote(ctx, pkg.registryAddr+"/"+pkgLayout.Pkg.Metadata.Name+":"+pkgLayout.Pkg.Metadata.Version, platform, oci.WithPlainHTTP(true))
	require.NoError(t, err)

	desc, err := remote.PushPackage(ctx, pkgLayout, zoci.PublishOptions{
		OCIConcurrency: 3,
		Retries:        1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, desc.Digest.String())
	require.Positive(t, desc.Size)

	fetchedRoot, err := remote.FetchRoot(ctx)
	require.NoError(t, err)
	require.Equal(t, zoci.ZarfConfigMediaType, fetchedRoot.Config.MediaType)

	configBytes, err := remote.FetchLayer(ctx, fetchedRoot.Config)
	require.NoError(t, err)
	var configPkg v1alpha1.ZarfPackage
	require.NoError(t, json.Unmarshal(configBytes, &configPkg))
	require.Equal(t, pkgLayout.Pkg.Metadata.Name, configPkg.Metadata.Name)
	require.Equal(t, pkgLayout.Pkg.Metadata.Version, configPkg.Metadata.Version)
}
