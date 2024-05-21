// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package bigbang

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequiredBigBangVersions(t *testing.T) {
	// Support 1.54.0 and beyond
	vv, err := isValidVersion("1.54.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Do not support earlier than 1.54.0
	vv, err = isValidVersion("1.53.0")
	require.NoError(t, err)
	require.False(t, vv)

	// Support for Big Bang release candidates
	vv, err = isValidVersion("1.57.0-rc.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Support for Big Bang 2.0.0
	vv, err = isValidVersion("2.0.0")
	require.NoError(t, err)
	require.True(t, vv)

	// Fail on non-semantic versions
	vv, err = isValidVersion("1.57b")
	require.EqualError(t, err, "Invalid Semantic Version")
	require.False(t, vv)
}
