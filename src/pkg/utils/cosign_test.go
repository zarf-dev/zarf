// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

	t.Run("empty options skip signing", func(t *testing.T) {
		require.False(t, SignBlobOptions{}.ShouldSign())
	})
}
