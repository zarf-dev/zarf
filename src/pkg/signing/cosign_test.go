// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultSignBlobOptions_EmptyAuthFlow guards against re-introducing a
// non-empty AuthFlow default. cosign's GetOAuthFlow treats any non-empty
// AuthFlow as an explicit override, which bypasses ambient OIDC provider
// detection (GitHub Actions, GCP, SPIFFE, etc.) entirely. Keep this empty.
func TestDefaultSignBlobOptions_EmptyAuthFlow(t *testing.T) {
	t.Parallel()
	opts := DefaultSignBlobOptions()
	require.Empty(t, opts.Fulcio.AuthFlow)
}

func TestShouldSign_KeyRefAlias(t *testing.T) {
	t.Parallel()

	t.Run("KeyRef alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.KeyRef = "/path/to/key"
		require.True(t, opts.ShouldSign())
	})

	t.Run("Key alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.Key = "/path/to/key"
		require.True(t, opts.ShouldSign())
	})

	t.Run("Keyless alone triggers signing", func(t *testing.T) {
		opts := SignBlobOptions{}
		opts.Keyless = true
		require.True(t, opts.ShouldSign())
	})

	t.Run("empty options skip signing", func(t *testing.T) {
		require.False(t, SignBlobOptions{}.ShouldSign())
	})
}
