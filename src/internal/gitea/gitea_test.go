// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package gitea

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	c, err := NewClient("https://example.com", "foo", "bar")
	require.NoError(t, err)
	require.Equal(t, "https", c.endpoint.Scheme)
	require.Equal(t, "foo", c.username)
	require.Equal(t, "bar", c.password)
}
