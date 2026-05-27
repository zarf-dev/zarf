// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package signing

import (
	"context"
	"testing"

	"github.com/sigstore/cosign/v3/pkg/providers"
	"github.com/stretchr/testify/require"
)

// TestGitHubActionsAmbientFlow validates that keyless signing works in CI
// environments that supply ambient OIDC credentials (e.g. GitHub Actions).
// The provider must be registered and enabled, and AuthFlow must be empty.
func TestGitHubActionsAmbientFlow(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "fake-token")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "https://token.actions.githubusercontent.com/fake")

	require.True(t, providers.Enabled(context.Background()))

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
