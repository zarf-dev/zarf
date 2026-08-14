// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestNewRemoteSetsZarfUserAgent(t *testing.T) {
	ctx := testutil.TestContext(t)
	originalCLIVersion := config.CLIVersion
	config.CLIVersion = "v1.2.3"
	t.Cleanup(func() {
		config.CLIVersion = originalCLIVersion
	})

	var observedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedUserAgent = r.UserAgent()
		w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
		w.Header().Set("Docker-Content-Digest", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		w.Header().Set("Content-Length", "2")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	registry := strings.TrimPrefix(server.URL, "http://")
	remote, err := zoci.NewRemote(ctx, registry+"/demo/some-package:1.0.0", zoci.PlatformForSkeleton(), oci.WithPlainHTTP(true))
	require.NoError(t, err)

	client, ok := remote.Repo().Client.(*auth.Client)
	require.True(t, ok)
	require.Equal(t, "zarf/v1.2.3", client.Header.Get("User-Agent"))

	_, err = remote.Repo().Resolve(ctx, "1.0.0")
	require.NoError(t, err)
	require.Equal(t, "zarf/v1.2.3", observedUserAgent)
}
