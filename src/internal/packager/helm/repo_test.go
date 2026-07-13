// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestNewChartRegistryClient(t *testing.T) {
	t.Run("does not create a client for an HTTP chart URL", func(t *testing.T) {
		client, plainHTTP, err := newChartRegistryClient(context.Background(), "https://charts.example.com/chart.tgz", types.RemoteOptions{})

		require.NoError(t, err)
		require.Nil(t, client)
		require.False(t, plainHTTP)
	})

	t.Run("creates a client for an OCI chart URL resolved from a Helm repository", func(t *testing.T) {
		client, plainHTTP, err := newChartRegistryClient(context.Background(), "oci://registry.example.com/charts/example:1.0.0", types.RemoteOptions{})

		require.NoError(t, err)
		require.NotNil(t, client)
		require.False(t, plainHTTP)
	})
}
