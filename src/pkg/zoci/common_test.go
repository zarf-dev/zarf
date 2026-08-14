// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestNewRemoteSetsZarfUserAgentOnFinalClient(t *testing.T) {
	ctx := testutil.TestContext(t)
	originalCLIVersion := config.CLIVersion
	config.CLIVersion = "v9.8.7"
	t.Cleanup(func() {
		config.CLIVersion = originalCLIVersion
	})

	remote, err := zoci.NewRemote(ctx, "registry.example.com/demo/dos-games:1.3.0", zoci.PlatformForSkeleton())
	require.NoError(t, err)

	client, ok := remote.Repo().Client.(*auth.Client)
	require.True(t, ok)
	require.Equal(t, "zarf/v9.8.7", client.Header.Get("User-Agent"))
}
